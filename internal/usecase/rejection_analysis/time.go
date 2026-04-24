package rejectionanalysis

import "time"

// nowUTC is overrideable in tests.
var nowUTC = func() time.Time { return time.Now().UTC() }
