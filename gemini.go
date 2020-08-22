package gemini

type geminiError string

func (e geminiError) Error() string { return string(e) }

// Gemini errors
const (
	ErrCert    = geminiError("certificate error")
	ErrFlush   = geminiError("already flushed")
	ErrHeader  = geminiError("invalid header")
	ErrRead    = geminiError("already called read")
	ErrRequest = geminiError("invalid request")
)

// Status represents a gemini status code
type Status int

// Gemini status codes, spec canonical names
const (
	StatusInput          Status = 10
	StatusInputSensitive        = 11

	StatusSuccess = 20

	StatusRedirect          = 30
	StatusRedirectTemporary = 30
	StatusRedirectPermanent = 31

	StatusTemporaryFailure  = 40
	StatusServerUnavailable = 41
	StatusCGIError          = 42
	StatusProxyError        = 43
	StatusSlowDown          = 44

	StatusPermanentFailure    = 50
	StatusNotFound            = 51
	StatusGone                = 52
	StatusProxyRequestRefused = 53
	StatusBadRequest          = 59

	StatusClientCertificateRequires = 60
	StatusCertificateNotAuthorized  = 61
	StatusCertificateNotValid       = 62
)

const MaxMeta = 1024
const MaxURL = 1024
