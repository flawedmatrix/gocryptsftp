package config

func SetTestPWReader() *FakePasswordReader {
	p := new(FakePasswordReader)
	pwReader = p
	return p
}
