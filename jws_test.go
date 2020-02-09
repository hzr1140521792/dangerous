package dangerous

import (
	"strings"
	"testing"
	"time"
)

var (
	jws = JSONWebSignatureSerializer{Secret: "secret-key", Expires_in: 10}
)

func Test_algorithm(t *testing.T) {
	for _, alg := range []string{"HS256", "HS384", "HS512", "none"} {
		_jws := jws
		_jws.AlgorithmName = alg
		data, _ := _jws.Dumps("value")
		if _, payload, err := _jws.Loads(string(data)); payload.(string) != "value" || err != nil {
			t.Fatalf("Algorithm is not available when inputed valid algorithm name. Algorithm:%s.", alg)
		}
	}
}

func Test_invalid_algorithm(t *testing.T) {
	_jws := jws
	_jws.AlgorithmName = "not exist"
	_jws.Dumps("value")
}

func Test_algorithm_mismatch(t *testing.T) {
	other := jws
	other.AlgorithmName = "HS256"
	signed, _ := other.Dumps("value")
	if _, _, err := jws.Loads(string(signed)); err == nil {
		t.Fatalf("Algorithm matched but expectation is mismatch.")
	}
}

// library can not report more error.
func Test_load_payload_exceptions(t *testing.T) {
	input := [][]string{
		{"ab", `does not match`},
		{"a.b", `does not match`},
		{"ew.b", `does not match`},
		{"ew.ab", `does not match`},
		{"W10.ab", `does not match`},
	}
	signer := jws.MakeSigner()
	for _, v := range input {
		signed := signer.Sign(v[0])
		_, _, err := jws.Loads(string(signed))
		if !strings.Contains(err.Error(), v[1]) {
			t.Fatalf("Unexpected error occured, we expect %s", v[1])
		}
	}

}

func Test_exp(t *testing.T) {
	jws.Expires_in = 8
	signed, _ := jws.TimedDumps("value")
	_, _, err := jws.TimedLoads(string(signed))
	if err != nil {
		t.Fatalf("Unexpected error occured when loads data. Error:%s", err.Error())
	}
	time.Sleep(10 * time.Second)
	_, value, err2 := jws.TimedLoads(string(signed))
	if err2 == nil {
		t.Fatalf("Load failed. Did not receive expected error.")
	}
	if value.(string) != "value" {
		t.Fatalf("Load failed. Incorrect output. %v", value)
	}
}

func Test_return_header(t *testing.T) {

}

func Test_missing_exp(t *testing.T) {
	header := jws.TimedMakeHeader(map[string]interface{}{})
	delete(header, "exp")
	signed, _ := jws.Dumps("value", header)

	_, _, err := jws.TimedLoads(string(signed))
	if !strings.Contains(err.Error(), `BadSignature`) {
		t.Fatalf("Load failed. Incorrect error: err%s", err.Error())
	}

}

func Test_invalid_exp(t *testing.T) {
	header := jws.TimedMakeHeader(map[string]interface{}{})
	header["exp"] = -1
	signed, _ := jws.Dumps("value", header)

	_, _, err := jws.TimedLoads(string(signed))
	if !strings.Contains(err.Error(), `BadHeader`) {
		t.Fatalf("Load failed. Incorrect error: err%s", err.Error())
	}

}
