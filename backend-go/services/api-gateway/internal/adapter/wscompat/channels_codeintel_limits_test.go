package wscompat

import (
	"testing"
	"time"
)

func TestCodeIntelTimeouts_ShorterThanInvokeTimeout(t *testing.T) {
	// 1. codeIntelReadTimeout < invokeTimeout, with margin >= 5s
	if codeIntelReadTimeout >= invokeTimeout {
		t.Errorf("codeIntelReadTimeout (%v) must be < invokeTimeout (%v)", codeIntelReadTimeout, invokeTimeout)
	}
	if invokeTimeout-codeIntelReadTimeout < 5*time.Second {
		t.Errorf("invokeTimeout - codeIntelReadTimeout must be >= 5s, got %v", invokeTimeout-codeIntelReadTimeout)
	}

	// 2. codeIntelStateTimeout == rpcTimeout
	if codeIntelStateTimeout != rpcTimeout {
		t.Errorf("codeIntelStateTimeout (%v) must equal rpcTimeout (%v)", codeIntelStateTimeout, rpcTimeout)
	}

	// 3. codeIntelSummaryTimeout < invokeTimeout, with margin >= 1s
	if codeIntelSummaryTimeout >= invokeTimeout {
		t.Errorf("codeIntelSummaryTimeout (%v) must be < invokeTimeout (%v)", codeIntelSummaryTimeout, invokeTimeout)
	}
	if invokeTimeout-codeIntelSummaryTimeout < 1*time.Second {
		t.Errorf("invokeTimeout - codeIntelSummaryTimeout must be >= 1s, got %v", invokeTimeout-codeIntelSummaryTimeout)
	}

	// 4. wsReadLimitBytes > 256 KiB + 8 KiB (enough for envelope overhead)
	const minRequired = (256 << 10) + (8 << 10)
	if wsReadLimitBytes <= minRequired {
		t.Errorf("wsReadLimitBytes (%d) must be > %d", wsReadLimitBytes, minRequired)
	}
}
