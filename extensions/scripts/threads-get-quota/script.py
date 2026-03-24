#!/usr/bin/env python3
# genetated by claude sonnet 4.6
import json
import sys
import urllib.parse
import urllib.request

import keyring

SERVICE = "agenvoy.threads"
BASE_URL = "https://graph.threads.net"


def get_credential(account: str) -> str:
    val = keyring.get_password(SERVICE, account)
    if not val:
        raise RuntimeError(
            f"Missing keychain entry: {SERVICE}/{account}. Run install_threads.sh first."
        )
    return val


def main():
    try:
        access_token = get_credential("access_token")
        user_id = get_credential("user_id")
    except RuntimeError as e:
        print(json.dumps({"error": str(e)}))
        return

    url = (
        f"{BASE_URL}/{user_id}/threads_publishing_limit"
        f"?fields=config,quota_usage"
        f"&access_token={urllib.parse.quote(access_token)}"
    )

    try:
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req, timeout=15) as resp:
            body = json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        error_body = json.loads(e.read().decode() or "{}")
        code = error_body.get("error", {}).get("code")
        print(
            json.dumps(
                {
                    "error": error_body.get("error", {}).get("message", str(e)),
                    "token_expired": code == 190,
                }
            )
        )
        return
    except Exception as e:
        print(json.dumps({"error": str(e)}))
        return

    data = (body.get("data") or [{}])[0]
    config = data.get("config", {})
    quota_usage = data.get("quota_usage", 0)
    quota_total = config.get("quota_total", 250)
    quota_duration = config.get("quota_duration", 86400)

    print(
        json.dumps(
            {
                "success": True,
                "used": quota_usage,
                "limit": quota_total,
                "remaining": quota_total - quota_usage,
                "window_seconds": quota_duration,
            }
        )
    )


if __name__ == "__main__":
    main()
