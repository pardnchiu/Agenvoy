#!/usr/bin/env python3
# genetated by claude sonnet 4.6
import json
import sys
import time
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
        get_credential("access_token")  # validate setup
        access_token = get_credential("access_token")
        app_secret = get_credential("app_secret")
    except RuntimeError as e:
        print(json.dumps({"error": str(e)}))
        return

    url = (
        f"{BASE_URL}/refresh_access_token"
        f"?grant_type=th_refresh_token"
        f"&access_token={urllib.parse.quote(access_token)}"
        f"&client_secret={urllib.parse.quote(app_secret)}"
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

    new_token = body.get("access_token")
    expires_in = body.get("expires_in", 0)

    if not new_token:
        print(json.dumps({"error": "No access_token in response", "response": body}))
        return

    try:
        keyring.set_password(SERVICE, "access_token", new_token)
    except Exception as e:
        print(json.dumps({"error": f"Keychain write failed: {e}"}))
        return

    print(
        json.dumps(
            {
                "success": True,
                "expires_in": expires_in,
                "expires_at": int(time.time()) + expires_in,
            }
        )
    )


if __name__ == "__main__":
    main()
