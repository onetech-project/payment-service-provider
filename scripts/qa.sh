#!/usr/bin/env bash
#
# One entry point for UAT testing. Wraps the per-endpoint scripts in this
# directory so a tester does not have to know which of them to call, which
# credential file each one wants, or how to carry a VA number from one step to
# the next.
#
# Usage:
#   ./scripts/qa.sh                      # interactive menu
#   ./scripts/qa.sh doctor               # check config, credentials, connectivity
#   ./scripts/qa.sh create -a 250000.00  # create a VA and remember it
#   ./scripts/qa.sh inquiry              # inquire the remembered VA
#   ./scripts/qa.sh pay                  # pay it
#   ./scripts/qa.sh status               # check its payment status
#   ./scripts/qa.sh help
#
# Three things it does that the underlying scripts cannot do for themselves:
#
#   1. Routes credentials. Merchant endpoints (create-va, list, delete) are
#      signed with a MERCHANT secret from .env.merchant.NAME; vendor endpoints
#      (inquiry, payment, status) with a VENDOR secret from
#      .env.<vendor>.<channel>. They are different credentials, the flag
#      spelling differs between scripts (-f, -e, -g), and swapping them
#      produces a 401 that reads like a signature bug. Here the right file is
#      chosen from the command.
#
#   2. Remembers the VA. `create` records partnerServiceId, customerNo,
#      virtualAccountNo and trxId, so `inquiry` / `pay` / `status` need no
#      arguments at all. `show` prints what is remembered, `use` points at an
#      existing VA, `reset` forgets it.
#
#   3. Keeps the target in one place. Base URL, credential files and the
#      default partnerServiceId live in scripts/.env.qa instead of being
#      retyped as flags on every call.
#
# Nothing here re-implements a request. Every command shells out to the same
# script a developer would run by hand, so what QA exercises is what the rest
# of this directory exercises.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

CONFIG_FILE="${QA_CONFIG:-${SCRIPT_DIR}/.env.qa}"
STATE_FILE="${QA_STATE:-${SCRIPT_DIR}/.qa-state}"

# --- output ------------------------------------------------------------------

step() { printf '\n\033[1;34m==>\033[0m \033[1m%s\033[0m\n' "$*"; }
info() { printf '    %s\n' "$*"; }
ok()   { printf '    \033[1;32mok\033[0m %s\n' "$*"; }
warn() { printf '    \033[1;33mwarning:\033[0m %s\n' "$*" >&2; }
die()  { printf '\n\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# --- config ------------------------------------------------------------------

if [[ -f "${CONFIG_FILE}" ]]; then
	set -a
	# shellcheck disable=SC1090
	source "${CONFIG_FILE}"
	set +a
fi

QA_BASE_URL="${QA_BASE_URL:-https://uatbca.manjo.co.id}"
QA_MERCHANT_ENV="${QA_MERCHANT_ENV:-.env.merchant.uatbca}"
QA_VENDOR_ENV="${QA_VENDOR_ENV:-.env.uatbca.va}"
# 15973/15974/15975 are reserved in master_partner_service_ids for the no-bill /
# variable-bill / fixed-bill VA types. A create-va against one of them is
# rejected unless it names a matching QA_VA_TYPE.
QA_PARTNER_SERVICE_ID="${QA_PARTNER_SERVICE_ID:-15975}"
QA_VA_TYPE="${QA_VA_TYPE:-03}"
QA_AMOUNT="${QA_AMOUNT:-250000.00}"
QA_VA_NAME="${QA_VA_NAME:-QA Tester}"
QA_LOG_DIR="${QA_LOG_DIR:-}"

# resolve <path> — makes a repo-relative credential path absolute, so the
# config file can name ".env.merchant.uatbca" and be run from anywhere.
resolve() {
	local p="$1"
	[[ "$p" == /* ]] && { printf '%s' "$p"; return; }
	printf '%s' "${ROOT_DIR}/${p}"
}

MERCHANT_ENV="$(resolve "${QA_MERCHANT_ENV}")"
VENDOR_ENV="$(resolve "${QA_VENDOR_ENV}")"

# --- credential guards -------------------------------------------------------

# The merchant and vendor files look alike at a glance and live side by side in
# the repo root, but carry different secrets. Passing one where the other
# belongs fails as "Unauthorized. [Invalid signature]", which sends a tester
# hunting for a signing bug that is not there. Checked by their key prefixes,
# which is the one thing that reliably tells the two apart.
require_merchant_env() {
	[[ -f "${MERCHANT_ENV}" ]] \
		|| die "no merchant credentials at ${MERCHANT_ENV}
       Set QA_MERCHANT_ENV in ${CONFIG_FILE}, or create the file with
       ./scripts/onboard-merchant.sh"
	grep -qE '^MERCHANT_(CLIENT_ID|SECRET_VALUE)=' "${MERCHANT_ENV}" \
		|| die "${MERCHANT_ENV} does not look like a merchant credentials file
       (no MERCHANT_* keys). This command signs with the MERCHANT secret; a
       .env.<vendor>.<channel> file here would fail as an invalid signature."
}

require_vendor_env() {
	[[ -f "${VENDOR_ENV}" ]] \
		|| die "no vendor credentials at ${VENDOR_ENV}
       Set QA_VENDOR_ENV in ${CONFIG_FILE}, or create the file with
       ./scripts/onboard-vendor.sh"
	grep -qE '^VENDOR_CLIENT_(ID|SECRET)=' "${VENDOR_ENV}" \
		|| die "${VENDOR_ENV} does not look like a vendor credentials file
       (no VENDOR_CLIENT_* keys). This command signs with the VENDOR secret; a
       .env.merchant.NAME file here would fail as an invalid signature."
}

# --- remembered VA -----------------------------------------------------------

ST_PARTNER_SERVICE_ID=""
ST_CUSTOMER_NO=""
ST_VA_NO=""
ST_TRX_ID=""
ST_INQUIRY_REQUEST_ID=""

load_state() {
	[[ -f "${STATE_FILE}" ]] || return 0
	# shellcheck disable=SC1090
	source "${STATE_FILE}"
}

save_state() {
	cat >"${STATE_FILE}" <<EOF
# Written by scripts/qa.sh. Delete this file, or run \`qa.sh reset\`, to forget
# the VA the inquiry/pay/status commands act on.
ST_PARTNER_SERVICE_ID='${ST_PARTNER_SERVICE_ID}'
ST_CUSTOMER_NO='${ST_CUSTOMER_NO}'
ST_VA_NO='${ST_VA_NO}'
ST_TRX_ID='${ST_TRX_ID}'
ST_INQUIRY_REQUEST_ID='${ST_INQUIRY_REQUEST_ID}'
EOF
}

require_va() {
	[[ -n "${ST_VA_NO}" ]] || die "no VA is selected.
       Run \`$0 create\` to make one, or \`$0 use -v <virtualAccountNo> -c <customerNo>\`
       to point at one that already exists."
}

# --- logging -----------------------------------------------------------------

# open_log <name> — when QA_LOG_DIR is set, every command's full output (both
# streams) is also written to a timestamped file, so a test run leaves evidence
# without the tester having to redirect anything.
#
# Colour escapes are stripped on the way into the file but kept on the
# terminal: a log that is going to be attached to a ticket should not be full
# of \e[1;34m. sed runs unbuffered so the last lines are not lost when the
# script exits before the process substitution flushes.
LOG_FILE=""
open_log() {
	[[ -n "${QA_LOG_DIR}" ]] || return 0
	mkdir -p "${QA_LOG_DIR}"
	LOG_FILE="${QA_LOG_DIR}/qa-$1-$(date +%Y%m%d-%H%M%S).log"
	if sed -u '' </dev/null >/dev/null 2>&1; then
		exec > >(tee >(sed -u 's/\x1b\[[0-9;]*m//g' >>"${LOG_FILE}")) 2>&1
	else
		exec > >(tee -a "${LOG_FILE}") 2>&1
	fi
}

# --- helpers -----------------------------------------------------------------

# A fresh 18-digit customerNo. The clock supplies uniqueness; $RANDOM covers
# two runs inside the same second, which second-resolution alone would collide.
generate_customer_no() {
	printf '%018d' "$(( 10#$(date +%y%m%d%H%M%S) * 100000 + RANDOM % 100000 ))"
}

# Extracts a field from a script's JSON stdout, or empty if absent.
json_field() { echo "$1" | jq -r "$2 // empty" 2>/dev/null || true; }

# Reports the responseCode of a captured response and returns non-zero when it
# is not a 2xx SNAP code, so a menu-driven run still ends in a visible failure.
report_code() {
	local response="$1" label="$2" code message
	code="$(json_field "${response}" '.responseCode')"
	message="$(json_field "${response}" '.responseMessage')"
	if [[ -z "${code}" ]]; then
		warn "${label}: no responseCode in the reply"
		return 1
	fi
	if [[ "${code}" == 2* ]]; then
		ok "${label}: ${code} ${message}"
		return 0
	fi
	warn "${label}: ${code} ${message}"
	return 1
}

# --- commands ----------------------------------------------------------------

cmd_doctor() {
	step "Checking the QA setup"

	local failed=0

	info "config file : ${CONFIG_FILE}$([[ -f "${CONFIG_FILE}" ]] || echo '  (absent — using defaults)')"
	info "base url    : ${QA_BASE_URL}"

	local tool
	for tool in curl openssl jq; do
		if command -v "${tool}" >/dev/null 2>&1; then
			ok "${tool} installed"
		else
			warn "${tool} is NOT installed — every command here needs it"
			failed=1
		fi
	done

	if [[ -f "${MERCHANT_ENV}" ]]; then
		if grep -qE '^MERCHANT_(CLIENT_ID|SECRET_VALUE)=' "${MERCHANT_ENV}"; then
			ok "merchant credentials: ${MERCHANT_ENV}"
			local key
			key="$(grep -E '^MERCHANT_PRIVATE_KEY_PATH=' "${MERCHANT_ENV}" | tail -n1 | cut -d= -f2- | tr -d '"'"'"'')"
			if [[ -n "${key}" && ! -f "${key}" ]]; then
				warn "  its MERCHANT_PRIVATE_KEY_PATH points at ${key}, which does not exist here"
				failed=1
			fi
		else
			warn "merchant credentials: ${MERCHANT_ENV} has no MERCHANT_* keys (wrong file?)"
			failed=1
		fi
	else
		warn "merchant credentials: ${MERCHANT_ENV} not found"
		failed=1
	fi

	if [[ -f "${VENDOR_ENV}" ]]; then
		if grep -qE '^VENDOR_CLIENT_(ID|SECRET)=' "${VENDOR_ENV}"; then
			ok "vendor credentials: ${VENDOR_ENV}"
			local vkey
			vkey="$(grep -E '^VENDOR_PRIVATE_KEY_PATH=' "${VENDOR_ENV}" | tail -n1 | cut -d= -f2- | tr -d '"'"'"'')"
			if [[ -n "${vkey}" && ! -f "${vkey}" ]]; then
				warn "  its VENDOR_PRIVATE_KEY_PATH points at ${vkey}, which does not exist here"
				failed=1
			fi
		else
			warn "vendor credentials: ${VENDOR_ENV} has no VENDOR_CLIENT_* keys (wrong file?)"
			failed=1
		fi
	else
		warn "vendor credentials: ${VENDOR_ENV} not found"
		failed=1
	fi

	local health
	if health="$(curl -sS -m 10 -o /dev/null -w '%{http_code}' "${QA_BASE_URL}/health" 2>&1)"; then
		if [[ "${health}" == "200" ]]; then
			ok "server reachable: ${QA_BASE_URL}/health -> 200"
		else
			warn "server answered ${health} at ${QA_BASE_URL}/health"
			failed=1
		fi
	else
		warn "cannot reach ${QA_BASE_URL}: ${health}"
		failed=1
	fi

	# partnerServiceId is String(8), space-padded on the left, in every BCA
	# field table. Reported rather than corrected: the reserved ids registered
	# in master_partner_service_ids are stored unpadded, so silently padding
	# here would stop matching them.
	info "partnerServiceId: '${QA_PARTNER_SERVICE_ID}' (${#QA_PARTNER_SERVICE_ID} chars)"
	if (( ${#QA_PARTNER_SERVICE_ID} != 8 )); then
		info "  BCA sends this left-padded to 8 characters. Set"
		info "  QA_PARTNER_SERVICE_ID='$(printf '%8s' "${QA_PARTNER_SERVICE_ID}")' to match that shape exactly."
	fi

	load_state
	if [[ -n "${ST_VA_NO}" ]]; then
		info "selected VA : ${ST_VA_NO}"
	else
		info "selected VA : <none> — run \`$0 create\` first"
	fi

	if (( failed )); then
		die "setup is incomplete — fix the warnings above before testing"
	fi
	step "Ready"
}

cmd_token() {
	require_vendor_env
	step "Fetching a B2B access token"
	local client_id key
	client_id="$(grep -E '^VENDOR_CLIENT_ID=' "${VENDOR_ENV}" | tail -n1 | cut -d= -f2- | tr -d '"'"'"'')"
	key="$(grep -E '^VENDOR_PRIVATE_KEY_PATH=' "${VENDOR_ENV}" | tail -n1 | cut -d= -f2- | tr -d '"'"'"'')"
	[[ -n "${client_id}" && -n "${key}" ]] \
		|| die "${VENDOR_ENV} needs both VENDOR_CLIENT_ID and VENDOR_PRIVATE_KEY_PATH to mint a token"
	"${SCRIPT_DIR}/curl-b2b-token.sh" -i "${client_id}" -p "${key}" -u "${QA_BASE_URL}"
}

cmd_create() {
	local amount="${QA_AMOUNT}" va_type="${QA_VA_TYPE}" name="${QA_VA_NAME}"
	local customer_no="" psid="${QA_PARTNER_SERVICE_ID}" expired=""
	while [[ $# -gt 0 ]]; do
		case "$1" in
			-a) amount="$2"; shift 2 ;;
			-y) va_type="$2"; shift 2 ;;
			-n) name="$2"; shift 2 ;;
			-c) customer_no="$2"; shift 2 ;;
			-s) psid="$2"; shift 2 ;;
			-x) expired="$2"; shift 2 ;;
			*) die "create: unknown option $1" ;;
		esac
	done

	require_merchant_env
	[[ -n "${customer_no}" ]] || customer_no="$(generate_customer_no)"

	step "Creating a VA (type ${va_type}, ${amount})"
	info "partnerServiceId : ${psid}"
	info "customerNo       : ${customer_no}"

	local -a args=(
		-s "${psid}" -c "${customer_no}" -n "${name}"
		-f "${MERCHANT_ENV}" -u "${QA_BASE_URL}" -y "${va_type}"
	)
	# A no-bill VA (01 static, 04 dynamic) carries no bill, and sending
	# totalAmount is rejected with 4002706.
	[[ "${va_type}" != "01" && "${va_type}" != "04" ]] && args+=(-a "${amount}")
	[[ -n "${expired}" ]] && args+=(-x "${expired}")

	local response
	response="$("${SCRIPT_DIR}/merchant-create-va.sh" "${args[@]}")"
	echo "${response}" | jq . 2>/dev/null || echo "${response}"

	report_code "${response}" "create-va" || die "create-va failed — nothing was remembered"

	load_state
	ST_PARTNER_SERVICE_ID="${psid}"
	ST_CUSTOMER_NO="$(json_field "${response}" '.virtualAccountData.customerNo')"
	ST_VA_NO="$(json_field "${response}" '.virtualAccountData.virtualAccountNo')"
	ST_TRX_ID="$(json_field "${response}" '.virtualAccountData.trxId')"
	ST_INQUIRY_REQUEST_ID=""
	# Fall back to what was sent when the reply omits a field, so the next
	# command still has something to work with.
	[[ -n "${ST_CUSTOMER_NO}" ]] || ST_CUSTOMER_NO="${customer_no}"
	[[ -n "${ST_VA_NO}" ]] || ST_VA_NO="${psid}${customer_no}"
	save_state

	step "Remembered"
	info "virtualAccountNo : ${ST_VA_NO}"
	info "trxId            : ${ST_TRX_ID:-<none>}"
	info "Next: $0 inquiry"
}

cmd_inquiry() {
	require_vendor_env
	load_state
	require_va

	step "Inquiring ${ST_VA_NO}"
	local response
	response="$("${SCRIPT_DIR}/vendor-inquiry-va.sh" \
		-s "${ST_PARTNER_SERVICE_ID}" -c "${ST_CUSTOMER_NO}" -v "${ST_VA_NO}" \
		-a "${QA_AMOUNT}" -f "${VENDOR_ENV}" -u "${QA_BASE_URL}")"
	echo "${response}" | jq . 2>/dev/null || echo "${response}"

	# BCA reuses inquiryRequestId as the payment's paymentRequestId when the
	# payment follows an inquiry, so it is kept for the pay step.
	local irid
	irid="$(json_field "${response}" '.virtualAccountData.inquiryRequestId')"
	if [[ -n "${irid}" ]]; then
		ST_INQUIRY_REQUEST_ID="${irid}"
		save_state
	fi

	report_code "${response}" "inquiry"
}

cmd_pay() {
	local amount="${QA_AMOUNT}" flag_advise=""
	while [[ $# -gt 0 ]]; do
		case "$1" in
			-a) amount="$2"; shift 2 ;;
			-A) flag_advise="$2"; shift 2 ;;
			*) die "pay: unknown option $1" ;;
		esac
	done

	require_vendor_env
	load_state
	require_va

	step "Paying ${ST_VA_NO} (${amount})"
	local -a args=(
		-s "${ST_PARTNER_SERVICE_ID}" -c "${ST_CUSTOMER_NO}" -v "${ST_VA_NO}"
		-a "${amount}" -n "${QA_VA_NAME}" -f "${VENDOR_ENV}" -u "${QA_BASE_URL}"
	)
	[[ -n "${flag_advise}" ]] && args+=(-A "${flag_advise}")

	local response
	response="$("${SCRIPT_DIR}/vendor-payment-va.sh" "${args[@]}")"
	echo "${response}" | jq . 2>/dev/null || echo "${response}"
	report_code "${response}" "payment"
}

cmd_status() {
	require_vendor_env
	load_state
	require_va

	step "Checking payment status of ${ST_VA_NO}"
	# The status service needs the inquiryRequestId the inquiry returned. Fall
	# back to the trxId when this VA was never inquired in this session.
	local rid="${ST_INQUIRY_REQUEST_ID:-${ST_TRX_ID}}"
	[[ -n "${rid}" ]] || die "no inquiryRequestId is remembered — run \`$0 inquiry\` first"

	"${SCRIPT_DIR}/aspi-simulator-request.sh" -e status -f "${VENDOR_ENV}" \
		-u "${QA_BASE_URL}" -s "${ST_PARTNER_SERVICE_ID}" -c "${ST_CUSTOMER_NO}" \
		-v "${ST_VA_NO}" -r "${rid}"
}

cmd_list_va() {
	require_merchant_env
	load_state
	step "Listing VAs for partnerServiceId ${ST_PARTNER_SERVICE_ID:-${QA_PARTNER_SERVICE_ID}}"
	"${SCRIPT_DIR}/merchant-list-va.sh" \
		-s "${ST_PARTNER_SERVICE_ID:-${QA_PARTNER_SERVICE_ID}}" \
		-f "${MERCHANT_ENV}" -u "${QA_BASE_URL}"
}

cmd_list_trx() {
	require_merchant_env
	load_state
	step "Listing transactions"
	local -a args=(-f "${MERCHANT_ENV}" -u "${QA_BASE_URL}")
	[[ -n "${ST_VA_NO}" ]] && args+=(-v "${ST_VA_NO}")
	args+=(-s "${ST_PARTNER_SERVICE_ID:-${QA_PARTNER_SERVICE_ID}}")
	"${SCRIPT_DIR}/merchant-list-transactions.sh" "${args[@]}"
}

cmd_delete() {
	require_merchant_env
	load_state
	require_va

	step "Deleting ${ST_VA_NO}"
	local -a args=(
		-s "${ST_PARTNER_SERVICE_ID}" -c "${ST_CUSTOMER_NO}" -v "${ST_VA_NO}"
		-f "${MERCHANT_ENV}" -u "${QA_BASE_URL}"
	)
	[[ -n "${ST_TRX_ID}" ]] && args+=(-t "${ST_TRX_ID}")
	"${SCRIPT_DIR}/merchant-delete-va.sh" "${args[@]}"

	cmd_reset
}

cmd_flow() {
	require_merchant_env
	require_vendor_env
	local customer_no
	customer_no="$(generate_customer_no)"
	step "Running the full create -> inquiry -> pay -> status flow"
	"${SCRIPT_DIR}/e2e-va-flow.sh" \
		-s "${QA_PARTNER_SERVICE_ID}" -c "${customer_no}" -n "${QA_VA_NAME}" \
		-a "${QA_AMOUNT}" -y "${QA_VA_TYPE}" \
		-m "${MERCHANT_ENV}" -f "${VENDOR_ENV}" -u "${QA_BASE_URL}" "$@"
}

cmd_negative() {
	require_merchant_env
	require_vendor_env
	step "Running the live negative-case suite"
	"${SCRIPT_DIR}/e2e-negative-cases.sh" \
		-m "${MERCHANT_ENV}" -f "${VENDOR_ENV}" -u "${QA_BASE_URL}" "$@"
}

cmd_request() {
	local endpoint="${1:-}"
	[[ -n "${endpoint}" ]] \
		|| die "request: name an endpoint — token, create-va, inquiry, payment, status or delete-va"
	shift

	# Merchant-signed endpoints take the merchant file; the rest the vendor one.
	local env_file
	case "${endpoint}" in
		create-va|delete-va) require_merchant_env; env_file="${MERCHANT_ENV}" ;;
		token|inquiry|payment|status) require_vendor_env; env_file="${VENDOR_ENV}" ;;
		*) die "request: unknown endpoint '${endpoint}'" ;;
	esac

	load_state
	step "Signed ${endpoint} request (copy-paste ready)"
	local -a args=(-e "${endpoint}" -f "${env_file}" -u "${QA_BASE_URL}" -a "${QA_AMOUNT}")
	if [[ -n "${ST_VA_NO}" ]]; then
		args+=(-s "${ST_PARTNER_SERVICE_ID}" -c "${ST_CUSTOMER_NO}" -v "${ST_VA_NO}")
		[[ -n "${ST_TRX_ID}" ]] && args+=(-t "${ST_TRX_ID}")
		[[ -n "${ST_INQUIRY_REQUEST_ID}" ]] && args+=(-r "${ST_INQUIRY_REQUEST_ID}")
	else
		args+=(-s "${QA_PARTNER_SERVICE_ID}")
	fi
	"${SCRIPT_DIR}/aspi-simulator-request.sh" "${args[@]}" "$@"
}

cmd_use() {
	load_state
	while [[ $# -gt 0 ]]; do
		case "$1" in
			-v) ST_VA_NO="$2"; shift 2 ;;
			-c) ST_CUSTOMER_NO="$2"; shift 2 ;;
			-s) ST_PARTNER_SERVICE_ID="$2"; shift 2 ;;
			-t) ST_TRX_ID="$2"; shift 2 ;;
			-r) ST_INQUIRY_REQUEST_ID="$2"; shift 2 ;;
			*) die "use: unknown option $1" ;;
		esac
	done
	[[ -n "${ST_VA_NO}" ]] || die "use: -v <virtualAccountNo> is required"
	# customerNo is the VA number minus the partnerServiceId prefix, so it can
	# be derived rather than demanded.
	if [[ -z "${ST_CUSTOMER_NO}" ]]; then
		ST_PARTNER_SERVICE_ID="${ST_PARTNER_SERVICE_ID:-${QA_PARTNER_SERVICE_ID}}"
		local trimmed_psid trimmed_va
		trimmed_psid="$(echo "${ST_PARTNER_SERVICE_ID}" | tr -d ' ')"
		trimmed_va="$(echo "${ST_VA_NO}" | tr -d ' ')"
		ST_CUSTOMER_NO="${trimmed_va#"${trimmed_psid}"}"
	fi
	save_state
	cmd_show
}

cmd_show() {
	load_state
	step "Selected VA"
	if [[ -z "${ST_VA_NO}" ]]; then
		info "<none> — run \`$0 create\`, or \`$0 use -v <virtualAccountNo>\`"
		return 0
	fi
	info "partnerServiceId : ${ST_PARTNER_SERVICE_ID}"
	info "customerNo       : ${ST_CUSTOMER_NO}"
	info "virtualAccountNo : ${ST_VA_NO}"
	info "trxId            : ${ST_TRX_ID:-<none>}"
	info "inquiryRequestId : ${ST_INQUIRY_REQUEST_ID:-<none, run inquiry>}"
}

cmd_reset() {
	rm -f "${STATE_FILE}"
	step "Forgot the selected VA"
}

cmd_help() {
	sed -n '2,39p' "${BASH_SOURCE[0]}" | sed 's/^#\s\?//'
	cat <<EOF

Commands:
  doctor            Check tools, credentials, connectivity and config
  create [-a amt] [-y type] [-n name] [-c customerNo] [-x expiry]
                    Create a VA and remember it
  inquiry           Inquire the remembered VA (vendor side)
  pay [-a amt] [-A Y|N]
                    Pay it (vendor side; -A Y is a retry/advice flag)
  status            Payment status of the remembered VA
  list-va           List the merchant's VAs
  list-trx          List transactions, filtered to the remembered VA
  delete            Delete the remembered VA, then forget it
  flow              Full create -> inquiry -> pay -> status, with assertions
  negative          The live negative-case suite
  request <ep>      Print a signed, copy-paste-ready request for Postman or the
                    ASPI simulator. <ep> is token|create-va|inquiry|payment|
                    status|delete-va
  show / use / reset
                    Inspect, set, or clear the remembered VA
                      use -v <virtualAccountNo> [-c <customerNo>] [-s <psid>]
  help              This text

Config (${CONFIG_FILE}), current values:
  QA_BASE_URL           ${QA_BASE_URL}
  QA_MERCHANT_ENV       ${QA_MERCHANT_ENV}
  QA_VENDOR_ENV         ${QA_VENDOR_ENV}
  QA_PARTNER_SERVICE_ID ${QA_PARTNER_SERVICE_ID}
  QA_VA_TYPE            ${QA_VA_TYPE}
  QA_AMOUNT             ${QA_AMOUNT}
  QA_VA_NAME            ${QA_VA_NAME}
  QA_LOG_DIR            ${QA_LOG_DIR:-<unset — no run logs written>}

VA types: 01 no-bill static, 02 variable static, 03 fixed static,
          04 no-bill dynamic, 05 variable dynamic, 06 fixed dynamic
EOF
}

# --- interactive menu --------------------------------------------------------

cmd_menu() {
	load_state
	while true; do
		printf '\n\033[1m  UAT test runner\033[0m — %s\n' "${QA_BASE_URL}"
		if [[ -n "${ST_VA_NO}" ]]; then
			printf '  selected VA: %s\n\n' "${ST_VA_NO}"
		else
			printf '  selected VA: <none>\n\n'
		fi
		printf '   1) doctor        check the setup\n'
		printf '   2) create        make a VA and select it\n'
		printf '   3) inquiry       inquire the selected VA\n'
		printf '   4) pay           pay it\n'
		printf '   5) status        its payment status\n'
		printf '   6) list-va       the merchant'"'"'s VAs\n'
		printf '   7) list-trx      transactions for the selected VA\n'
		printf '   8) delete        delete the selected VA\n'
		printf '   9) flow          full create -> inquiry -> pay -> status\n'
		printf '  10) negative      live negative-case suite\n'
		printf '  11) request       print a signed request for Postman\n'
		printf '  12) show          what is selected\n'
		printf '  13) reset         forget the selected VA\n'
		printf '   q) quit\n\n'

		local choice
		read -rp "  choice: " choice || { echo; return 0; }

		# A failing step must not drop the tester out of the menu — the whole
		# point is to try the next thing after seeing a rejection.
		case "${choice}" in
			1) cmd_doctor || true ;;
			2) cmd_create || true ;;
			3) cmd_inquiry || true ;;
			4) cmd_pay || true ;;
			5) cmd_status || true ;;
			6) cmd_list_va || true ;;
			7) cmd_list_trx || true ;;
			8) cmd_delete || true ;;
			9) cmd_flow || true ;;
			10) cmd_negative || true ;;
			11)
				local ep
				read -rp "  endpoint (token|create-va|inquiry|payment|status|delete-va): " ep || true
				[[ -n "${ep}" ]] && { cmd_request "${ep}" || true; }
				;;
			12) cmd_show || true ;;
			13) cmd_reset || true ;;
			q|Q|"") return 0 ;;
			*) warn "no such choice: ${choice}" ;;
		esac
		load_state
	done
}

# --- dispatch ----------------------------------------------------------------

command -v jq >/dev/null 2>&1 || die "jq is not installed — run \`$0 doctor\` for the full list"

COMMAND="${1:-menu}"
[[ $# -gt 0 ]] && shift

case "${COMMAND}" in
	menu)       cmd_menu ;;
	doctor)     open_log doctor;   cmd_doctor ;;
	token)      open_log token;    cmd_token ;;
	create)     open_log create;   cmd_create "$@" ;;
	inquiry)    open_log inquiry;  cmd_inquiry ;;
	pay)        open_log pay;      cmd_pay "$@" ;;
	status)     open_log status;   cmd_status ;;
	list-va)    open_log list-va;  cmd_list_va ;;
	list-trx)   open_log list-trx; cmd_list_trx ;;
	delete)     open_log delete;   cmd_delete ;;
	flow)       open_log flow;     cmd_flow "$@" ;;
	negative)   open_log negative; cmd_negative "$@" ;;
	request)    cmd_request "$@" ;;
	show)       cmd_show ;;
	use)        cmd_use "$@" ;;
	reset)      cmd_reset ;;
	help|-h|--help) cmd_help ;;
	*) die "unknown command: ${COMMAND}
       Run \`$0 help\` for the list, or \`$0\` for the menu." ;;
esac
