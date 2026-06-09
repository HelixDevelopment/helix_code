# HXC-049 — doc_processor TestAutomation_UpstreamsExist case-drift (CONST-052)
Found by owned-submodule health sweep (D-2). automation_test.go:140 read os.ReadDir("Upstreams") (capital) but the
canonical dir is lowercase "upstreams" (CONST-052) → deterministic FAIL every run. FIX: "Upstreams"→"upstreams" + msgs.
GREEN: go test -run TestAutomation_UpstreamsExist . → ok digital.vasic.docprocessor 0.170s. doc_processor commit ecb384f.
