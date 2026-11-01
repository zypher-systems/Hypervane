#!/bin/sh
# Writes nginx's list of reverse proxies from HYPERVANE_TRUSTED_PROXIES, a comma
# or space separated list of addresses or CIDR ranges. nginx accepts a client
# address from X-Forwarded-For only when the request arrives from one of these,
# so an empty list means every client is taken at its connecting address.
set -eu

out="${TRUSTED_PROXIES_CONF:-/etc/nginx/trusted-proxies.conf}"
list=$(printf '%s' "${HYPERVANE_TRUSTED_PROXIES:-}" | tr ',' ' ')
rules=""

for entry in $list; do
	case "$entry" in
	*[!0-9a-fA-F:./]*)
		echo "HYPERVANE_TRUSTED_PROXIES: '$entry' is not an address or CIDR range" >&2
		exit 1
		;;
	*/0)
		echo "HYPERVANE_TRUSTED_PROXIES: '$entry' trusts every client; list only your proxies" >&2
		exit 1
		;;
	esac
	rules="${rules}set_real_ip_from ${entry};
"
done

printf '%s' "$rules" >"$out"
