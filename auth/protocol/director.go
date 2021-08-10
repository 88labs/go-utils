//go:generate mockgen -source=$GOFILE -destination=mock/$GOFILE -package=mock_$GOPACKAGE

package protocol

import "net/http"

type Director interface {
	SetUrl(w http.ResponseWriter, r *http.Request, redirectUrl string) error
	GetUrl(w http.ResponseWriter, r *http.Request) string
	GetUrlFromParams(r *http.Request) string
}
