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


def api_post(url: str, data: dict) -> dict:
    encoded = urllib.parse.urlencode(data).encode()
    req = urllib.request.Request(url, data=encoded, method="POST")
    req.add_header("Content-Type", "application/x-www-form-urlencoded")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        error_body = json.loads(e.read().decode() or "{}")
        code = error_body.get("error", {}).get("code")
        raise _APIError(
            error_body.get("error", {}).get("message", str(e)),
            token_expired=(code == 190),
        )


class _APIError(Exception):
    def __init__(self, message: str, token_expired: bool = False):
        super().__init__(message)
        self.token_expired = token_expired


def main():
    try:
        params = json.loads(sys.stdin.read() or "{}")
    except json.JSONDecodeError:
        print(json.dumps({"error": "Invalid JSON input"}))
        return

    image_url = (params.get("image_url") or "").strip()
    text = (params.get("text") or "").strip()

    if not image_url:
        print(json.dumps({"error": "image_url is required"}))
        return
    if text and len(text) > 500:
        print(json.dumps({"error": f"text exceeds 500 characters ({len(text)})"}))
        return

    try:
        access_token = get_credential("access_token")
        user_id = get_credential("user_id")
    except RuntimeError as e:
        print(json.dumps({"error": str(e)}))
        return

    try:
        container_data = {
            "media_type": "IMAGE",
            "image_url": image_url,
            "access_token": access_token,
        }
        if text:
            container_data["text"] = text

        container = api_post(f"{BASE_URL}/{user_id}/threads", container_data)
        container_id = container.get("id")
        if not container_id:
            print(
                json.dumps(
                    {"error": "Failed to create container", "response": container}
                )
            )
            return

        result = api_post(
            f"{BASE_URL}/{user_id}/threads_publish",
            {"creation_id": container_id, "access_token": access_token},
        )
    except _APIError as e:
        print(json.dumps({"error": str(e), "token_expired": e.token_expired}))
        return
    except Exception as e:
        print(json.dumps({"error": str(e)}))
        return

    media_id = result.get("id")
    if not media_id:
        print(json.dumps({"error": "Publish failed", "response": result}))
        return

    print(json.dumps({"success": True, "media_id": media_id}))


if __name__ == "__main__":
    main()
