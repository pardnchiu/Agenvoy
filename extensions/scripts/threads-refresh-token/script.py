#!/usr/bin/env python3
# genetated by claude sonnet 4.6
import json
import sys
import time
import urllib.parse
import urllib.request

AGENVOY_API = "http://localhost:17989"
BASE_URL = "https://graph.threads.net"


def get_credential(account: str) -> str:
    key = f"agenvoy.threads.{account}"
    url = f"{AGENVOY_API}/v1/key?key={urllib.parse.quote(key)}"
    try:
        with urllib.request.urlopen(url, timeout=5) as resp:
            val = json.loads(resp.read().decode()).get("value", "")
    except Exception:
        val = ""
    if not val:
        raise RuntimeError(f"Missing key: {key}. Run install_threads.sh first.")
    return val


def set_credential(account: str, value: str) -> None:
    key = f"agenvoy.threads.{account}"
    data = json.dumps({"key": key, "value": value}).encode()
    req = urllib.request.Request(
        f"{AGENVOY_API}/v1/key",
        data=data,
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    urllib.request.urlopen(req, timeout=5)


def main():
    try:
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
        set_credential("access_token", new_token)
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
