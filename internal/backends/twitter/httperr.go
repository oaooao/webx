package twitter

import (
	"errors"
	"net/url"
)

// requestErrorString describes a failed HTTP round trip without echoing the
// request URL's query. GraphQL GETs carry kilobytes of URL-encoded features
// and variables in the query; net/http's *url.Error repeats all of it, which
// buries the actual cause ("connection reset by peer") at the end of a huge
// line. The operation path (which names the GraphQL operation) is kept.
func requestErrorString(err error) string {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err.Error()
	}
	target := ue.URL
	if u, perr := url.Parse(ue.URL); perr == nil {
		u.RawQuery = ""
		u.Fragment = ""
		target = u.String()
	}
	return ue.Op + " " + target + ": " + ue.Err.Error()
}
