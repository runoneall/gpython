# M3 生成器测试：yield / next / StopIteration / 生成器表达式


def count_up(limit):
    value = 1
    while value <= limit:
        yield value
        value = value + 1


assert list(count_up(3)) == [1, 2, 3]
assert list(count_up(0)) == []

gen = count_up(2)
assert next(gen) == 1
assert next(gen) == 2
try:
    next(gen)
    assert False, "expected StopIteration"
except StopIteration:
    pass


# 生成器表达式
gen_squares = (x * x for x in range(1, 4))
assert list(gen_squares) == [1, 4, 9]


# 带 yield 的斐波那契
def fib(n):
    a, b = 0, 1
    for _ in range(n):
        yield a
        a, b = b, a + b


assert list(fib(6)) == [0, 1, 1, 2, 3, 5]

print("m3_generator: all checks passed")
