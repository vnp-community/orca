package domain

import (
	"context"
	"errors"
	"strings"
)

// Git reports a failed fetch only as text on stderr, so the classes a caller must tell apart (the ref does not exist, the
// remote could not be reached) are recognised from that text, the same pragmatic approach isBaseRefNotFoundErr already takes.
// A translated git (LANG other than English) falls through to "unclassified", which callers treat as a failure that says
// nothing about the ref, never as "the ref is missing".

var remoteRefMissingMarkers = []string{
	"couldn't find remote ref",
	"invalid refspec",
	// A repository without an origin has no remote to hold the ref: only its local refs count, and those were already tried.
	"'origin' does not appear to be a git repository",
	"no such remote",
}

var unreachableMarkers = []string{
	"could not resolve host",
	"could not read from remote repository",
	"unable to access",
	"connection refused",
	"connection timed out",
	"connection reset",
	"operation timed out",
	"network is unreachable",
	"no route to host",
	"early eof",
	"the remote end hung up",
	"permission denied (publickey)",
	"authentication failed",
	"ssh: connect to host",
	"i/o timeout",
	"context deadline exceeded",
	"code = unavailable",
	"code = deadlineexceeded",
}

// IsRemoteRefMissing reports whether a failed `git fetch origin <ref>` says the ref is not there.
func IsRemoteRefMissing(err error) bool { return errorTextHas(err, remoteRefMissingMarkers) }

// IsRemoteUnreachable reports whether a failed git call could not reach the remote or the dev server, so it says nothing
// about the ref. A missing ref is checked first by callers: git prints "Could not read from remote repository" after both.
func IsRemoteUnreachable(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errorTextHas(err, unreachableMarkers)
}

func errorTextHas(err error, markers []string) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, m := range markers {
		if strings.Contains(text, m) {
			return true
		}
	}
	return false
}
