package v1

import "testing"

func TestLimits(t *testing.T) {
	if MaxRequestBytes != 1<<20 || MaxResponseBytes != 16<<20 || MaxRequestIDLen != 128 || MaxTimeoutMS != 600000 || MaxErrorMessageBytes != 4096 {
		t.Fatal("wire limits changed")
	}
}
