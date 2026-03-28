#!/usr/bin/env python3
# genetated by claude sonnet 4.6
import json
import sys
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
