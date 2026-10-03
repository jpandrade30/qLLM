"""Mint a qLLM derived key (same format as internal/access/derived.go)."""

from __future__ import annotations

import base64
import hashlib
import hmac
import time

KNOWN_VECTOR = "mobile.42.1767225600.PyV9Flp-KOKj1RxuR0Oa0CTEbPI3QFTTfw-fI-kXVbI"


def mint_key(app: str, user_code: str, secret: str, expiry_unix: int | None = None, ttl_s: int = 3600) -> str:
    if expiry_unix is None:
        expiry_unix = int(time.time()) + ttl_s
    msg = f"{app}.{user_code}.{expiry_unix}"
    digest = hmac.new(secret.encode("utf-8"), msg.encode("utf-8"), hashlib.sha256).digest()
    sig = base64.urlsafe_b64encode(digest).decode("ascii").rstrip("=")
    return f"{msg}.{sig}"
