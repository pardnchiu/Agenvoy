#!/usr/bin/env python3
import json
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

BASE_URL = "https://api.coinlore.net"
TIMEOUT_SECONDS = 20
MAX_RETRIES = 3


def main():
    args = json.loads(sys.stdin.read() or "{}")

    query = args.get("query")
    if not isinstance(query, str) or not query.strip():
        print("missing required parameter: query", file=sys.stderr)
        sys.exit(1)

    include_matches = bool(args.get("include_matches", False))

    try:
        result = lookup_price(query.strip(), include_matches)
    except Exception as e:
        print(f"failed: {e}", file=sys.stderr)
        sys.exit(1)

    print(json.dumps(result, separators=(",", ":")))


def lookup_price(query, include_matches):
    if query.isdigit():
        coin_id = query
        matches = []
        lookup_method = "id"
    else:
        assets_response = fetch_json("/api/assets/")
        assets = extract_assets(assets_response)
        matches = find_asset_matches(assets, query)
        if not matches:
            raise RuntimeError(f"no cryptocurrency found for query: {query}")
        best = sorted(matches, key=lambda item: parse_rank(item.get("rank")))[0]
        coin_id = str(best.get("id", "")).strip()
        if not coin_id:
            raise RuntimeError("matched asset has no CoinLore ID")
        lookup_method = "asset_lookup"

    ticker = fetch_ticker(coin_id)
    result = {
        "query": query,
        "lookup_method": lookup_method,
        "source": "CoinLore",
        "source_url": "https://www.coinlore.com/cryptocurrency-data-api",
        "coin": normalize_ticker(ticker),
    }
    if include_matches:
        result["matches"] = [normalize_asset(asset) for asset in matches[:10]]
    return result


def fetch_ticker(coin_id):
    data = fetch_json("/api/ticker/", {"id": coin_id})
    if not isinstance(data, list) or not data:
        raise RuntimeError(f"no ticker data found for CoinLore ID: {coin_id}")
    if not isinstance(data[0], dict):
        raise RuntimeError("unexpected ticker response")
    return data[0]


def extract_assets(response):
    if isinstance(response, list):
        return response
    if isinstance(response, dict) and isinstance(response.get("data"), list):
        return response["data"]
    raise RuntimeError("unexpected assets response")


def find_asset_matches(assets, query):
    needle = query.casefold()
    exact = []
    contains = []

    for asset in assets:
        if not isinstance(asset, dict):
            continue
        symbol = str(asset.get("symbol", "")).casefold()
        name = str(asset.get("name", "")).casefold()
        nameid = str(asset.get("nameid", "")).casefold()

        if needle in {symbol, name, nameid}:
            exact.append(asset)
        elif needle and (needle in symbol or needle in name or needle in nameid):
            contains.append(asset)

    return exact or contains


def fetch_json(path, params=None):
    url = BASE_URL + path
    if params:
        url += "?" + urllib.parse.urlencode(params)

    last_error = None
    for attempt in range(MAX_RETRIES):
        try:
            request = urllib.request.Request(url, headers={"User-Agent": "agenvoy-fetch-crypto-price/1.0"})
            with urllib.request.urlopen(request, timeout=TIMEOUT_SECONDS) as response:
                status = getattr(response, "status", 200)
                if status < 200 or status >= 300:
                    raise RuntimeError(f"HTTP {status}")
                charset = response.headers.get_content_charset() or "utf-8"
                return json.loads(response.read().decode(charset))
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError, RuntimeError) as e:
            last_error = e
            if attempt + 1 < MAX_RETRIES:
                time.sleep(0.5 * (attempt + 1))

    raise RuntimeError(f"request failed for {url}: {last_error}")


def normalize_asset(asset):
    return {
        "id": as_string(asset.get("id")),
        "symbol": as_string(asset.get("symbol")),
        "name": as_string(asset.get("name")),
        "nameid": as_string(asset.get("nameid")),
        "rank": parse_optional_int(asset.get("rank")),
    }


def normalize_ticker(ticker):
    return {
        "id": as_string(ticker.get("id")),
        "symbol": as_string(ticker.get("symbol")),
        "name": as_string(ticker.get("name")),
        "nameid": as_string(ticker.get("nameid")),
        "rank": parse_optional_int(ticker.get("rank")),
        "price_usd": parse_optional_float(ticker.get("price_usd")),
        "price_btc": parse_optional_float(ticker.get("price_btc")),
        "percent_change_1h": parse_optional_float(ticker.get("percent_change_1h")),
        "percent_change_24h": parse_optional_float(ticker.get("percent_change_24h")),
        "percent_change_7d": parse_optional_float(ticker.get("percent_change_7d")),
        "market_cap_usd": parse_optional_float(ticker.get("market_cap_usd")),
        "volume_24h_usd": parse_optional_float(ticker.get("volume24")),
        "circulating_supply": parse_optional_float(ticker.get("csupply")),
        "total_supply": parse_optional_float(ticker.get("tsupply")),
        "max_supply": parse_optional_float(ticker.get("msupply")),
    }


def as_string(value):
    return "" if value is None else str(value)


def parse_rank(value):
    parsed = parse_optional_int(value)
    return parsed if parsed is not None else 10**12


def parse_optional_int(value):
    if value in (None, ""):
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def parse_optional_float(value):
    if value in (None, ""):
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


if __name__ == "__main__":
    main()
