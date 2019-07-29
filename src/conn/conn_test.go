package conn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateLoadSession(t *testing.T) {
	client := &Conn{
		Addr: "127.0.0.1",
		Port: 5566,
	}
	_, err := client.NewUpdateSession("../1.json", 0)
	assert.NotNil(t, err, nil)
}
