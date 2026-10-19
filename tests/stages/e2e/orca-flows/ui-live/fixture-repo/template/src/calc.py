def multiply(a, b):
    """Multiplies two numbers."""
    return a * b


def max_of(a, b):
    """Returns the greater of two numbers."""
    return a if a > b else b


def clamp(n, lo, hi):
    """Limits n to the range lo..hi."""
    if n < lo:
        return lo
    if n > hi:
        return lo
    return n
