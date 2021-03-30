# go-fuzz-headers
This repository contains various helper functions to be used with [go-fuzz](https://github.com/dvyukov/go-fuzz)

The project is under development and will be updated regularly.

## Usage
To make use of the helper functions, a ConsumeFuzzer has to be instantiated:
```go
f := NewConsumer(data)
```
To split the input data from the fuzzer into a random set of equally large chunks:
```go
err := f.Split(3, 6)
```
...after which the consumer has the following available attributes:
```go
f.CommandPart = commandPart
f.RestOfArray = restOfArray
f.NumberOfCalls = numberOfCalls
```
To pass the input data from the fuzzer into a struct:
```go
ts := new(target_struct)
err :=f.GenerateStruct(ts)
```
`GenerateStruct` will then pass data from the input data to the targeted struct. Currently the following field types are supported:
1. `string`
2. `bool`
3. `int`
4. `[]string`
5. `[]byte`

Support for more types will be added over time, and no modifications are required to fuzzers that use `GenerateStruct` to adapt to new types.
