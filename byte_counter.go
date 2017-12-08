package main

type byteCounter struct {
	size int64
}

func (c *byteCounter) Write(p []byte) (n int, err error) {
	c.size += int64(len(p))
	return len(p), nil
}
