package prelude

type Middleware func(next HandlerFunc) HandlerFunc

const DefaultContentType = "application/json"
