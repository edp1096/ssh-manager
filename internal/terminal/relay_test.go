package terminal

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRelayLaunchArguments(t *testing.T) {
	if len(relayArguments(SshClientArgument{})) != 0 {
		t.Fatal("standalone launch changed")
	}
	arg := SshClientArgument{RelayAddress: "127.0.0.1:1234", RelayToken: "test-token"}
	if !reflect.DeepEqual(relayArguments(arg), []string{"-relay-address", "127.0.0.1:1234", "-relay-token", "test-token"}) {
		t.Fatal("incorrect relay arguments")
	}
	var request SshClientArgument
	if err := json.Unmarshal([]byte(`{"RelayAddress":"external:22","RelayToken":"forged"}`), &request); err != nil {
		t.Fatal(err)
	}
	if len(relayArguments(request)) != 0 {
		t.Fatal("browser can override private relay credentials")
	}
}
