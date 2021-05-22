package gofuzzheaders

import (
	"errors"
	"reflect"
)

type ConsumeFuzzer struct {
	data          []byte
	CommandPart   []byte
	RestOfArray   []byte
	NumberOfCalls int
	position      int
}

func IsDivisibleBy(n int, divisibleby int) bool {
	return (n % divisibleby) == 0
}

func NewConsumer(fuzzData []byte) *ConsumeFuzzer {
	f := &ConsumeFuzzer{data: fuzzData, position: 0}
	return f
}

func (f *ConsumeFuzzer) Split(minCalls, maxCalls int) error {
	if len(f.data) == 0 {
		return errors.New("Could not split")
	}
	numberOfCalls := int(f.data[0])
	if numberOfCalls < minCalls || numberOfCalls > maxCalls {
		return errors.New("Bad number of calls")

	}
	if len(f.data) < numberOfCalls+numberOfCalls+1 {
		return errors.New("Length of data does not match required parameters")
	}

	// Define part 2 and 3 of the data array
	commandPart := f.data[1 : numberOfCalls+1]
	restOfArray := f.data[numberOfCalls+1:]

	// Just a small check. It is necessary
	if len(commandPart) != numberOfCalls {
		return errors.New("Length of commandPart does not match number of calls")
	}

	// Check if restOfArray is divisible by numberOfCalls
	if !IsDivisibleBy(len(restOfArray), numberOfCalls) {
		return errors.New("Length of commandPart does not match number of calls")
	}
	f.CommandPart = commandPart
	f.RestOfArray = restOfArray
	f.NumberOfCalls = numberOfCalls
	return nil
}

func (f *ConsumeFuzzer) GenerateStruct(targetStruct interface{}) error {
	v := reflect.ValueOf(targetStruct)
	v = v.Elem()
	err := f.fuzzStruct(v)
	if err != nil {
		return err
	}
	return nil
}

func (f *ConsumeFuzzer) fuzzStruct(e reflect.Value) error {
    switch e.Kind() {
    case reflect.Struct:
        for i := 0; i < e.NumField(); i++ {
            err := f.fuzzStruct(e.Field(i))
            if err != nil {
                return err
            }
        }
    case reflect.String:
        str, err := f.GetString()
        if err != nil {
            return err
        }
        if e.CanSet() {
            e.SetString(str)
        }
    case reflect.Slice:
        uu := reflect.MakeSlice(e.Type(), 5, 5)
        for i := 0; i < 5; i++ {
            err := f.fuzzStruct(uu.Index(i))
            if err != nil {
                return err
            }
        }
        if e.CanSet() {
            e.Set(uu)
        }
    case reflect.Uint64:
        
        newInt, err := f.GetInt()
        if err != nil {
            return err
        }
        e.SetUint(uint64(newInt))
    case reflect.Int:
        
        newInt, err := f.GetInt()
        if err != nil {
            return err
        }
        e.SetInt(int64(newInt))        
    case reflect.Map:
        if e.CanSet() {
            e.Set(reflect.MakeMap(e.Type()))
            for i := 0; i < 5; i++ {
                key := reflect.New(e.Type().Key()).Elem()
                err := f.fuzzStruct(key)
                if err != nil {
                    return err
                }
                val := reflect.New(e.Type().Elem()).Elem()
                err = f.fuzzStruct(val)
                if err != nil {
                    return err
                }
                e.SetMapIndex(key, val)
            }
        }
    default:
        return nil
    }
    return nil
}


func (f *ConsumeFuzzer) OldGenerateStruct(targetStruct interface{}) error {
	if f.position >= len(f.data) {
		return errors.New("Not enough bytes to proceed")
	}
	e := reflect.ValueOf(targetStruct).Elem()
	for i := 0; i < e.NumField(); i++ {
		fieldtype := e.Type().Field(i).Type.String()
		switch ft := fieldtype; ft {
		case "string":
			stringChunk, err := f.GetString()
			if err != nil {
				return err
			}
			e.Field(i).SetString(stringChunk)
		case "bool":
			newBool, err := f.GetBool()
			if err != nil {
				return err
			}
			e.Field(i).SetBool(newBool)
		case "int":
			newInt, err := f.GetInt()
			if err != nil {
				return err
			}
			e.Field(i).SetInt(int64(newInt))
		case "[]string":
			newStringArray, err := f.GetStringArray()
			if err != nil {
				return err
			}
			if e.Field(i).CanSet() {
				e.Field(i).Set(newStringArray)
			}
		case "[]byte":
			newBytes, err := f.GetBytes()
			if err != nil {
				return err
			}
			e.Field(i).SetBytes(newBytes)
		default:
			continue
		}
	}
	return nil
}

func (f *ConsumeFuzzer) GetStringArray() (reflect.Value, error) {
	// The max size of the array:
	max := 20

	arraySize := f.position
	if arraySize > max {
		arraySize = max
	}
	elemType := reflect.TypeOf("string")
	stringArray := reflect.MakeSlice(reflect.SliceOf(elemType), arraySize, arraySize)
	if f.position+arraySize >= len(f.data) {
		return stringArray, errors.New("Could not make string array")
	}

	for i := 0; i < arraySize; i++ {
		stringSize := int(f.data[f.position])

		if f.position+stringSize >= len(f.data) {
			return stringArray, nil
		}
		stringToAppend := string(f.data[f.position : f.position+stringSize])
		strVal := reflect.ValueOf(stringToAppend)
		stringArray = reflect.Append(stringArray, strVal)
		f.position = f.position + stringSize
	}
	return stringArray, nil
}

func (f *ConsumeFuzzer) GetInt() (int, error) {
	if f.position >= len(f.data) {
		return 0, errors.New("Not enough bytes to create int")
	}
	returnInt := int(f.data[f.position])
	f.position++
	return returnInt, nil
}

func (f *ConsumeFuzzer) GetBytes() ([]byte, error) {
	if f.position >= len(f.data) {
		return nil, errors.New("Not enough bytes to create byte array")
	}
	length := int(f.data[f.position])
	if length == 0 {
		return nil, errors.New("Zero-length is not supported")
	}
	if f.position+length >= len(f.data) {
		return nil, errors.New("Not enough bytes to create byte array")
	}
	b := f.data[f.position : f.position+length]
	f.position = f.position + length
	return b, nil
}

func (f *ConsumeFuzzer) GetString() (string, error) {
	if f.position >= len(f.data) {
		return "nil", errors.New("Not enough bytes to create string")
	}
	length := int(f.data[f.position])
	if f.position+length >= len(f.data) {
		return "nil", errors.New("Not enough bytes to create string")
	}
	str := string(f.data[f.position : f.position+length])
	f.position = f.position + length
	return str, nil
}

func (f *ConsumeFuzzer) GetBool() (bool, error) {
	if f.position >= len(f.data) {
		return false, errors.New("Not enough bytes to create bool")
	}
	if IsDivisibleBy(int(f.data[f.position]), 2) {
		f.position++
		return true, nil
	} else {
		f.position++
		return false, nil
	}
}
