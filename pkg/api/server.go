package api

import "context"

type Frontend interface {
	Serve(context.Context)
}
