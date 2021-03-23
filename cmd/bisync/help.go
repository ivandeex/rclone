package bisync

import (
	"strconv"
	"strings"
)

func makeHelp(help string) string {
	replacer := strings.NewReplacer(
		"|", "`",
		"{MAXDELETE}", strconv.Itoa(DefaultMaxDelete),
		"{CHECKFILE}", DefaultCheckFilename,
		"{WORKDIR}", DefaultWorkdir,
	)
	return replacer.Replace(help)
}

var shortHelp string = `Perform bidirectonal synchronization between two paths.`

var rcHelp string = makeHelp(`
TODO
`)

var longHelp string = shortHelp + makeHelp(`
TODO
`)
