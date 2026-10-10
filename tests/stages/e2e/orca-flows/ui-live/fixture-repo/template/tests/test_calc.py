import unittest

from src.calc import clamp, max_of, multiply


class CalcTest(unittest.TestCase):
    def test_multiply_multiplies(self):
        self.assertEqual(multiply(3, 4), 12)

    def test_max_of_returns_the_greater_number(self):
        self.assertEqual(max_of(2, 9), 9)
        self.assertEqual(max_of(5, 1), 5)

    def test_clamp_keeps_a_number_that_is_already_in_range(self):
        self.assertEqual(clamp(5, 0, 10), 5)
        self.assertEqual(clamp(-3, 0, 10), 0)


if __name__ == "__main__":
    unittest.main()
