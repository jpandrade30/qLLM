from mint import KNOWN_VECTOR, mint_key


def test_known_vector_matches_go():
    assert mint_key("mobile", "42", "test-secret", expiry_unix=1767225600) == KNOWN_VECTOR
