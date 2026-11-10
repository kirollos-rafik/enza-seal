// Package clipboard wraps the system clipboard. Read and Write are variables
// so tests can replace them.
package clipboard

import atotto "github.com/atotto/clipboard"

var (
	Read  = atotto.ReadAll
	Write = atotto.WriteAll
)
