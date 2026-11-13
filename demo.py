# gpython acceptance demo.
# This file describes the expected final behaviour of the interpreter.
# Run with: go run . demo.py
#
# Expected output is documented at the bottom of this file.
#
# async/await are explicitly NOT supported: parsing either one is a fatal
# error that exits immediately and cannot be caught by try/except.


import math
import math as m
from math import sqrt
from math import factorial as fact
import os
import sys
import threading
import subprocess

print("1. imports")
print("   floor(3.7) =", math.floor(3.7))
print("   pi =", math.pi)
print("   sqrt(81) =", sqrt(81))
print("   6! =", fact(6))
print("   alias works =", m.floor(2.9))
print("   os.sep =", os.sep)
print("   sys.maxsize =", sys.maxsize)

assert math.floor(3.7) == 3
assert sqrt(81) == 9
assert fact(6) == 720
assert m.floor(2.9) == 2
assert isinstance(math.pi, float)
assert os.sep == "/"
assert sys.maxsize == 9223372036854775807

print("2. def")


def greet(name, greeting="hello"):
    return greeting + ", " + name


def pos_only(a, /, b=2):
    return a + b


def total(*args, scale=1, **kwargs):
    result = 0
    for value in args:
        result = result + value
    for key in kwargs:
        result = result + kwargs[key]
    return result * scale


def fibonacci(n):
    if n < 2:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)


def make_adder(amount):
    def add(value):
        return value + amount

    return add


def double_result(fn):
    def wrapper(value):
        return fn(value) * 2

    return wrapper


@double_result
def increment(value):
    return value + 1


def count_up(limit):
    value = 1
    while value <= limit:
        yield value
        value = value + 1


def evens(values):
    for value in values:
        if value % 2 == 0:
            yield value


print("   " + greet("world"))
print("   " + greet("gpython", "hi"))
print("   pos_only(1) =", pos_only(1))
print("   total =", total(1, 2, 3, scale=2, bonus=4))
print("   fibonacci(10) =", fibonacci(10))
add_ten = make_adder(10)
print("   closure =", add_ten(5))
print("   decorated =", increment(7))
print("   count_up(3) =", list(count_up(3)))
print("   evens =", list(evens([1, 2, 3, 4])))
generator = count_up(2)
print("   next =", next(generator))
square = lambda value: value * value
add_together = lambda left, right=1: left + right
classify = lambda value: "big" if value >= 10 else "small"
print("   lambda =", square(6), add_together(4, 6))
print("   lambda ternary =", classify(12), classify(3))

assert greet("world") == "hello, world"
assert greet("gpython", "hi") == "hi, gpython"
assert pos_only(1) == 3
assert pos_only(1, 5) == 6
assert total(1, 2, 3, scale=2, bonus=4) == 20
assert fibonacci(10) == 55
assert add_ten(5) == 15
assert increment(7) == 16
assert list(count_up(3)) == [1, 2, 3]
assert list(evens([1, 2, 3, 4])) == [2, 4]
assert next(count_up(5)) == 1

stopped = False
generator = count_up(1)
try:
    next(generator)
    next(generator)
except StopIteration:
    stopped = True
assert stopped

assert square(6) == 36
assert add_together(4) == 5
assert add_together(4, 6) == 10
assert classify(12) == "big"
assert classify(3) == "small"

print("3. class")


class Animal:
    kind = "animal"

    def __init__(self, name, sound):
        self.name = name
        self.sound = sound

    def speak(self):
        return self.name + " says " + self.sound

    def describe(self):
        return self.kind + ":" + self.name


class Named:
    def label(self):
        return "named:" + self.name


class Dog(Animal, Named):
    kind = "dog"

    def __init__(self, name):
        super().__init__(name, "woof")

    def speak(self):
        return super().speak() + "!"


class Counter:
    created = 0

    def __init__(self):
        Counter.created = Counter.created + 1
        self.number = Counter.created


animal = Animal("cat", "meow")
dog = Dog("rex")
first = Counter()
second = Counter()

print("   " + animal.speak())
print("   " + animal.describe())
print("   " + dog.speak())
print("   " + dog.label())
print("   counter =", second.number)

assert animal.speak() == "cat says meow"
assert animal.describe() == "animal:cat"
assert dog.kind == "dog"
assert dog.speak() == "rex says woof!"
assert dog.label() == "named:rex"
assert isinstance(dog, Animal)
assert isinstance(dog, Named)
assert first.number == 1
assert second.number == 2
assert Counter.created == 2

print("4. try/except")


class ValidationError(Exception):
    pass


class Account:
    def __init__(self, balance):
        self.balance = balance

    def withdraw(self, amount):
        if amount > self.balance:
            raise ValidationError("balance too low")
        self.balance = self.balance - amount
        return self.balance


events = []


def safe_divide(a, b):
    try:
        result = a / b
    except ZeroDivisionError as exc:
        return "caught: " + str(exc)
    except (TypeError, ValueError):
        return "bad operands"
    else:
        return result
    finally:
        events.append("safe_divide")


divide_ok = safe_divide(8, 2)
divide_zero = safe_divide(1, 0)
divide_bad = safe_divide("x", 2)

print("   " + str(divide_ok))
print("   " + divide_zero)
print("   " + divide_bad)
print("   finally count =", len(events))

assert divide_ok == 4.0
assert divide_zero == "caught: division by zero"
assert divide_bad == "bad operands"
assert len(events) == 3

account = Account(100)
try:
    account.withdraw(150)
except ValidationError as exc:
    print("   caught validation:", str(exc))
else:
    print("   unexpected success")
finally:
    print("   account unchanged:", account.balance)

assert account.balance == 100

results = []

try:
    raise ValueError
except ValueError:
    results.append("raise class")

try:
    raise RuntimeError("boom")
except:
    results.append("bare except")

try:
    raise KeyError("k")
except (TypeError, KeyError):
    results.append("tuple except")

try:
    text = int("not-a-number")
except ValueError:
    results.append("int valueerror")

try:
    value = [1, 2][5]
except IndexError:
    results.append("indexerror")

try:
    value = {"a": 1}["b"]
except KeyError:
    results.append("keyerror")

try:
    value = 1 + "x"
except TypeError:
    results.append("typeerror")

try:
    value = missing_name
except NameError:
    results.append("nameerror")

not_callable = 1
try:
    not_callable()
except TypeError:
    results.append("not callable")

assert results == [
    "raise class",
    "bare except",
    "tuple except",
    "int valueerror",
    "indexerror",
    "keyerror",
    "typeerror",
    "nameerror",
    "not callable",
]
print("   error kinds =", len(results))

chain = []
try:
    try:
        raise ValueError("inner")
    finally:
        chain.append("inner finally")
except ValueError:
    chain.append("outer except")

assert chain == ["inner finally", "outer except"]

messages = []
try:
    try:
        raise ValueError("cause")
    except ValueError as cause:
        raise ValidationError("wrapper") from cause
except ValidationError as exc:
    messages.append(str(exc))

try:
    try:
        raise ValidationError("original")
    except ValidationError:
        raise
except ValidationError as exc:
    messages.append(str(exc))

assert messages == ["wrapper", "original"]


def finally_wins():
    try:
        return "try"
    finally:
        return "finally"


assert finally_wins() == "finally"

else_events = []
try:
    else_events.append("try")
except:
    else_events.append("except")
else:
    else_events.append("else")
finally:
    else_events.append("finally")

assert else_events == ["try", "else", "finally"]

print("5. control flow and builtins")

total_value = 0
for number in range(1, 6):
    total_value += number

index = 0
seen = []
while index < 6:
    index += 1
    if index == 2:
        continue
    if index == 5:
        break
    seen.append(index)

items = [1, 2, 3]
items.append(4)
squares = {}
for value in items:
    squares[value] = value * value

a, b = 10, 20
score = 75
grade = "pass" if score >= 60 else "fail"
name = "gpython"

print("   sum 1..5 =", total_value)
print("   seen =", seen)
print("   items =", items)
print("   slice =", items[1:3])
print("   squares =", squares)
print("   grade =", grade)

assert total_value == 15
assert seen == [1, 3, 4]
assert items == [1, 2, 3, 4]
assert items[1:3] == [2, 3]
assert items[-1] == 4
assert squares[4] == 16
assert a + b == 30
assert 0 < score <= 100
assert grade == "pass"
assert (True and not False) or False
assert len(items) == 4
assert sum(items) == 10
assert sorted([3, 1, 2]) == [1, 2, 3]
assert name[0] == "g"
assert name[1:4] == "pyt"
assert name[-1] == "n"
assert len(name) == 7
assert name + "!" == "gpython!"
assert "py" in name

print("6. strings")

raw = r"c:\tmp\new"
fstring = f"{name!r} has {len(name)} chars, pi={math.pi:.2f}"
percent = "%s=%d" % (name, 7)
mapping = "%(who)s scored %(points)03d" % {"who": name, "points": 7}
literal = "100%%" % ()

print("   raw =", raw)
print("   f-string =", fstring)
print("   percent =", percent)
print("   mapping =", mapping)
print("   literal =", literal)

assert raw == "c:\\tmp\\new"
assert fstring == "'gpython' has 7 chars, pi=3.14"
assert percent == "gpython=7"
assert mapping == "gpython scored 007"
assert literal == "100%"

print("7. comprehensions")

squares_list = [value * value for value in range(1, 6)]
evens_set = {value for value in range(1, 7) if value % 2 == 0}
square_map = {value: value * value for value in range(1, 4)}
pairs = [(x, y) for x in range(1, 3) for y in range(1, 3)]
filtered = [value for value in range(10) if value % 2 == 0 if value > 2]
gen_squares = (value * value for value in range(1, 4))
gen_list = list(gen_squares)

print("   list =", squares_list)
print("   set =", evens_set)
print("   dict =", square_map)
print("   nested =", pairs)
print("   filtered =", filtered)
print("   generator =", gen_list)

assert squares_list == [1, 4, 9, 16, 25]
assert evens_set == {2, 4, 6}
assert square_map == {1: 1, 2: 4, 3: 9}
assert pairs == [(1, 1), (1, 2), (2, 1), (2, 2)]
assert filtered == [4, 6, 8]
assert gen_list == [1, 4, 9]

print("8. threading")

results = []


def worker(value):
    results.append(value)


t = threading.Thread(target=worker, args=(42,))
t.start()
t.join()
print("   thread joined:", results == [42])
print("   thread alive after join:", not t.is_alive())

lock = threading.Lock()
lock.acquire()
print("   lock locked:", lock.locked())
lock.release()
print("   lock locked after release:", not lock.locked())

event = threading.Event()
print("   event initially set:", not event.is_set())
event.set()
print("   event set:", event.is_set())
event.clear()
print("   event cleared:", not event.is_set())

print("   active_count:", threading.active_count())
print("   main thread:", threading.current_thread().name)

assert results == [42]
assert not t.is_alive()
assert threading.active_count() == 1
assert threading.current_thread().name == "MainThread"

print("9. subprocess")

cp = subprocess.run(["echo", "hello"], capture_output=True)
print("   subprocess rc:", cp.returncode)
assert cp.returncode == 0
assert cp.stdout == "hello\n"

p = subprocess.Popen(["echo", "popen"])
out, err = p.communicate()
print("   popen rc:", p.returncode)
assert p.returncode == 0
assert out == "popen\n"

print("all acceptance checks passed")

# ---------------------------------------------------------------------------
# Expected output (go run . demo.py):
# 1. imports
#    floor(3.7) = 3
#    pi = 3.141592653589793
#    sqrt(81) = 9.0
#    6! = 720
#    alias works = 2
#    os.sep = /
#    sys.maxsize = 9223372036854775807
# 2. def
#    hello, world
#    hi, gpython
#    pos_only(1) = 3
#    total = 20
#    fibonacci(10) = 55
#    closure = 15
#    decorated = 16
#    count_up(3) = [1, 2, 3]
#    evens = [2, 4]
#    next = 1
#    lambda = 36 10
#    lambda ternary = big small
# 3. class
#    cat says meow
#    animal:cat
#    rex says woof!
#    named:rex
#    counter = 2
# 4. try/except
#    4.0
#    caught: division by zero
#    bad operands
#    finally count = 3
#    caught validation: balance too low
#    account unchanged: 100
#    error kinds = 9
# 5. control flow and builtins
#    sum 1..5 = 15
#    seen = [1, 3, 4]
#    items = [1, 2, 3, 4]
#    slice = [2, 3]
#    squares = {1: 1, 2: 4, 3: 9, 4: 16}
#    grade = pass
# 6. strings
#    raw = c:\tmp\new
#    f-string = 'gpython' has 7 chars, pi=3.14
#    percent = gpython=7
#    mapping = gpython scored 007
#    literal = 100%
# 7. comprehensions
#    list = [1, 4, 9, 16, 25]
#    set = {2, 4, 6}
#    dict = {1: 1, 2: 4, 3: 9}
#    nested = [(1, 1), (1, 2), (2, 1), (2, 2)]
#    filtered = [4, 6, 8]
#    generator = [1, 4, 9]
# 8. threading
#    thread joined: True
#    thread alive after join: True
#    lock locked: True
#    lock locked after release: True
#    event initially set: True
#    event set: True
#    event cleared: True
#    active_count: 1
#    main thread: MainThread
# 9. subprocess
#    subprocess rc: 0
#    popen rc: 0
# all acceptance checks passed
# ---------------------------------------------------------------------------
