# M6 lambda 与三元运算符测试

square = lambda x: x * x
assert square(6) == 36

add = lambda a, b=1: a + b
assert add(4) == 5
assert add(4, 6) == 10

classify = lambda v: "big" if v >= 10 else "small"
assert classify(12) == "big"
assert classify(3) == "small"

# 三元表达式直接使用
x = 5
result = "yes" if x > 0 else "no"
assert result == "yes"

y = -1
result = "yes" if y > 0 else "no"
assert result == "no"


# lambda 闭包捕获
def make_multiplier(factor):
    return lambda x: x * factor


times3 = make_multiplier(3)
assert times3(4) == 12

print("m6_lambda_ternary: all checks passed")
