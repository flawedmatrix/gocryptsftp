// Code generated by counterfeiter. DO NOT EDIT.
package config

import (
	"sync"
)

type FakePasswordReader struct {
	ReadPasswordStub        func(string) ([]byte, error)
	readPasswordMutex       sync.RWMutex
	readPasswordArgsForCall []struct {
		arg1 string
	}
	readPasswordReturns struct {
		result1 []byte
		result2 error
	}
	readPasswordReturnsOnCall map[int]struct {
		result1 []byte
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakePasswordReader) ReadPassword(arg1 string) ([]byte, error) {
	fake.readPasswordMutex.Lock()
	ret, specificReturn := fake.readPasswordReturnsOnCall[len(fake.readPasswordArgsForCall)]
	fake.readPasswordArgsForCall = append(fake.readPasswordArgsForCall, struct {
		arg1 string
	}{arg1})
	fake.recordInvocation("ReadPassword", []interface{}{arg1})
	fake.readPasswordMutex.Unlock()
	if fake.ReadPasswordStub != nil {
		return fake.ReadPasswordStub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.readPasswordReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePasswordReader) ReadPasswordCallCount() int {
	fake.readPasswordMutex.RLock()
	defer fake.readPasswordMutex.RUnlock()
	return len(fake.readPasswordArgsForCall)
}

func (fake *FakePasswordReader) ReadPasswordCalls(stub func(string) ([]byte, error)) {
	fake.readPasswordMutex.Lock()
	defer fake.readPasswordMutex.Unlock()
	fake.ReadPasswordStub = stub
}

func (fake *FakePasswordReader) ReadPasswordArgsForCall(i int) string {
	fake.readPasswordMutex.RLock()
	defer fake.readPasswordMutex.RUnlock()
	argsForCall := fake.readPasswordArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakePasswordReader) ReadPasswordReturns(result1 []byte, result2 error) {
	fake.readPasswordMutex.Lock()
	defer fake.readPasswordMutex.Unlock()
	fake.ReadPasswordStub = nil
	fake.readPasswordReturns = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakePasswordReader) ReadPasswordReturnsOnCall(i int, result1 []byte, result2 error) {
	fake.readPasswordMutex.Lock()
	defer fake.readPasswordMutex.Unlock()
	fake.ReadPasswordStub = nil
	if fake.readPasswordReturnsOnCall == nil {
		fake.readPasswordReturnsOnCall = make(map[int]struct {
			result1 []byte
			result2 error
		})
	}
	fake.readPasswordReturnsOnCall[i] = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakePasswordReader) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.readPasswordMutex.RLock()
	defer fake.readPasswordMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakePasswordReader) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ PasswordReader = new(FakePasswordReader)
