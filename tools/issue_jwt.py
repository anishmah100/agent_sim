#!/usr/bin/env python3
"""Issue an HS256 JWT compatible with the engine's `-jwt-secret` flag.

For friends-only launches without a real auth backend. Set the same
secret on the engine and use this CLI to mint per-friend tokens.

Usage:
    python3 tools/issue_jwt.py --secret "$JWT_SECRET" \
        --subject alice@example.com --ttl-days 30
"""
from __future__ import annotations

import argparse
import base64
import hashlib
import hmac
import json
import sys
import time
import os


def _b64url(b: bytes) -> str:
    return base64.urlsafe_b64encode(b).rstrip(b"=").decode("ascii")


def sign(payload: dict, secret: bytes) -> str:
    header = {"alg": "HS256", "typ": "JWT"}
    h = _b64url(json.dumps(header, separators=(",", ":")).encode())
    p = _b64url(json.dumps(payload, separators=(",", ":")).encode())
    sig = hmac.new(secret, f"{h}.{p}".encode(), hashlib.sha256).digest()
    return f"{h}.{p}.{_b64url(sig)}"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--secret", default=os.environ.get("JWT_SECRET", ""),
                        help="HS256 secret matching the engine's -jwt-secret flag")
    parser.add_argument("--subject", required=True, help="subject claim (user id / email)")
    parser.add_argument("--ttl-days", type=int, default=30, help="token lifetime in days")
    parser.add_argument("--extra", default="{}",
                        help="JSON object of additional claims to embed")
    args = parser.parse_args()

    if not args.secret:
        sys.exit("error: pass --secret or set JWT_SECRET env var")

    extra = json.loads(args.extra)
    payload = {
        "sub": args.subject,
        "iat": int(time.time()),
        "exp": int(time.time() + args.ttl_days * 86400),
        **extra,
    }
    print(sign(payload, args.secret.encode()))


if __name__ == "__main__":
    main()
