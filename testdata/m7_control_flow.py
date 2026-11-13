# M7 控制流测试：if/elif/else / for / while / break / continue

# if/elif/else 链
score = 75
if score >= 90:
    grade = "A"
elif score >= 80:
    grade = "B"
elif score >= 70:
    grade = "C"
else:
    grade = "D"
assert grade == "C"

# for + break/continue
seen = []
for i in range(1, 7):
    if i == 2:
        continue
    if i == 5:
        break
    seen.append(i)
assert seen == [1, 3, 4]

# while 循环
total = 0
n = 1
while n <= 5:
    total = total + n
    n = n + 1
assert total == 15

# 嵌套 for + if
nested = []
for i in range(3):
    for j in range(3):
        if i == j:
            nested.append((i, j))
assert nested == [(0, 0), (1, 1), (2, 2)]

# while + break
counted = []
k = 0
while True:
    k = k + 1
    if k > 4:
        break
    counted.append(k)
assert counted == [1, 2, 3, 4]

print("m7_control_flow: all checks passed")
