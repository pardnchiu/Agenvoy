#!/usr/bin/env python3
import json
import sys
import urllib.request


def main():
    args = json.loads(sys.stdin.read() or "{}")

    lat = args.get("latitude")
    lon = args.get("longitude")
    if lat is None or lon is None:
        print("missing required parameter: latitude and longitude", file=sys.stderr)
        sys.exit(1)

    url = (
        f"https://api.open-meteo.com/v1/forecast"
        f"?latitude={lat}&longitude={lon}"
        f"&current=temperature_2m,relative_humidity_2m,apparent_temperature,"
        f"precipitation,weather_code,wind_speed_10m,wind_direction_10m"
        f"&timezone=auto"
    )

    try:
        req = urllib.request.Request(url, headers={"User-Agent": "agenvoy/1.0"})
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = json.loads(resp.read().decode())
    except Exception as e:
        print(f"failed to fetch weather: {e}", file=sys.stderr)
        sys.exit(1)

    current = data.get("current", {})
    units = data.get("current_units", {})
    tz = data.get("timezone", "")

    wmo = {
        0: "Clear sky", 1: "Mainly clear", 2: "Partly cloudy", 3: "Overcast",
        45: "Fog", 48: "Depositing rime fog",
        51: "Light drizzle", 53: "Moderate drizzle", 55: "Dense drizzle",
        61: "Slight rain", 63: "Moderate rain", 65: "Heavy rain",
        71: "Slight snow", 73: "Moderate snow", 75: "Heavy snow",
        80: "Slight rain showers", 81: "Moderate rain showers", 82: "Violent rain showers",
        95: "Thunderstorm", 96: "Thunderstorm with slight hail", 99: "Thunderstorm with heavy hail",
    }

    code = current.get("weather_code", -1)

    result = {
        "timezone": tz,
        "time": current.get("time", ""),
        "condition": wmo.get(code, f"Unknown ({code})"),
        "temperature_c": current.get("temperature_2m"),
        "apparent_temperature_c": current.get("apparent_temperature"),
        "humidity_pct": current.get("relative_humidity_2m"),
        "precipitation_mm": current.get("precipitation"),
        "wind_speed_kmh": current.get("wind_speed_10m"),
        "wind_direction_deg": current.get("wind_direction_10m"),
    }

    print(json.dumps(result))


if __name__ == "__main__":
    main()
