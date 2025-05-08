package service

import (
	"github.com/pentops/grpc.go/protovalidatemw"
	"github.com/pentops/log.go/grpc_log"
	"github.com/pentops/log.go/log"
	"github.com/pentops/realms/j5auth"
	"google.golang.org/grpc"
)

func GRPCUnaryMiddleware(version string, validateReply bool) []grpc.UnaryServerInterceptor {
	var pvOpts []protovalidatemw.Option
	if validateReply {
		pvOpts = append(pvOpts, protovalidatemw.WithReply())
	}
	return []grpc.UnaryServerInterceptor{
		grpc_log.UnaryServerInterceptor(log.DefaultContext, log.DefaultTrace, log.DefaultLogger),
		j5auth.GRPCMiddleware,
		protovalidatemw.UnaryServerInterceptor(pvOpts...),
	}
}
