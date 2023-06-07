package common

import (
	"context"
	"github.com/orca-zhang/ecache/dist"
	"github.com/redis/go-redis/v9"
)

type GoRedisCli struct {
	ctx      context.Context
	redisCli *redis.Client
	chanSize int
}

func (g *GoRedisCli) OK() bool {
	_, err := g.redisCli.Ping(g.ctx).Result()
	return err == nil
}

func (g *GoRedisCli) Pub(channel, payload string) error {
	_, err := g.redisCli.Publish(g.ctx, channel, payload).Result()
	return err
}

func (g *GoRedisCli) Sub(channel string, callback func(payload string)) error {
	msgChan := g.redisCli.Subscribe(g.ctx, channel).Channel(redis.WithChannelSize(g.chanSize))
	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				return nil
			}
			callback(msg.Payload)
		default:
		}
	}
}

func Take(r *redis.Client, size ...int) dist.RedisCli {
	s := 100 // default 100 messages
	if len(size) > 0 {
		s = size[0]
	}
	return &GoRedisCli{
		ctx:      context.TODO(),
		redisCli: r,
		chanSize: s,
	}
}
