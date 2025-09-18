package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/presence-service/internal/biz"
)

type presenceRepo struct {
	redis *redis.Client
}

func NewPresenceRepo(redis *redis.Client) biz.PresenceRepo {
	return &presenceRepo{redis: redis}
}

const (
	userPresencePrefix    = "presence:user:"
	deviceSessionPrefix   = "session:device:"
	userSessionsPrefix    = "sessions:user:"
	presenceExpiration    = 24 * time.Hour
	sessionExpiration     = 24 * time.Hour
)

func (r *presenceRepo) SetUserPresence(ctx context.Context, presence *biz.UserPresence) error {
	key := fmt.Sprintf("%s%s", userPresencePrefix, presence.UserID.String())
	
	data, err := json.Marshal(presence)
	if err != nil {
		return err
	}

	return r.redis.Set(ctx, key, data, presenceExpiration).Err()
}

func (r *presenceRepo) GetUserPresence(ctx context.Context, userID uuid.UUID) (*biz.UserPresence, error) {
	key := fmt.Sprintf("%s%s", userPresencePrefix, userID.String())
	
	data, err := r.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return &biz.UserPresence{
			UserID:   userID,
			Status:   biz.StatusOffline,
			LastSeen: time.Now(),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var presence biz.UserPresence
	if err := json.Unmarshal([]byte(data), &presence); err != nil {
		return nil, err
	}

	return &presence, nil
}

func (r *presenceRepo) GetMultipleUserPresence(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]*biz.UserPresence, error) {
	if len(userIDs) == 0 {
		return make(map[uuid.UUID]*biz.UserPresence), nil
	}

	keys := make([]string, len(userIDs))
	for i, userID := range userIDs {
		keys[i] = fmt.Sprintf("%s%s", userPresencePrefix, userID.String())
	}

	results, err := r.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	presenceMap := make(map[uuid.UUID]*biz.UserPresence)
	for i, result := range results {
		userID := userIDs[i]
		
		if result == nil {
			// User not found in cache, set as offline
			presenceMap[userID] = &biz.UserPresence{
				UserID:   userID,
				Status:   biz.StatusOffline,
				LastSeen: time.Now(),
			}
			continue
		}

		var presence biz.UserPresence
		if err := json.Unmarshal([]byte(result.(string)), &presence); err != nil {
			// On error, set as offline
			presenceMap[userID] = &biz.UserPresence{
				UserID:   userID,
				Status:   biz.StatusOffline,
				LastSeen: time.Now(),
			}
			continue
		}

		presenceMap[userID] = &presence
	}

	return presenceMap, nil
}

func (r *presenceRepo) CreateDeviceSession(ctx context.Context, session *biz.DeviceSession) error {
	sessionKey := fmt.Sprintf("%s%s", deviceSessionPrefix, session.ClientID)
	userSessionsKey := fmt.Sprintf("%s%s", userSessionsPrefix, session.UserID.String())

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	pipe := r.redis.Pipeline()
	pipe.Set(ctx, sessionKey, data, sessionExpiration)
	pipe.SAdd(ctx, userSessionsKey, session.ClientID)
	pipe.Expire(ctx, userSessionsKey, sessionExpiration)

	_, err = pipe.Exec(ctx)
	return err
}

func (r *presenceRepo) UpdateDeviceSession(ctx context.Context, session *biz.DeviceSession) error {
	sessionKey := fmt.Sprintf("%s%s", deviceSessionPrefix, session.ClientID)
	
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	return r.redis.Set(ctx, sessionKey, data, sessionExpiration).Err()
}

func (r *presenceRepo) GetDeviceSession(ctx context.Context, clientID string) (*biz.DeviceSession, error) {
	sessionKey := fmt.Sprintf("%s%s", deviceSessionPrefix, clientID)
	
	data, err := r.redis.Get(ctx, sessionKey).Result()
	if err == redis.Nil {
		return nil, biz.ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	var session biz.DeviceSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *presenceRepo) GetUserDeviceSessions(ctx context.Context, userID uuid.UUID) ([]*biz.DeviceSession, error) {
	userSessionsKey := fmt.Sprintf("%s%s", userSessionsPrefix, userID.String())
	
	clientIDs, err := r.redis.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return nil, err
	}

	if len(clientIDs) == 0 {
		return []*biz.DeviceSession{}, nil
	}

	sessions := make([]*biz.DeviceSession, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		session, err := r.GetDeviceSession(ctx, clientID)
		if err == nil {
			sessions = append(sessions, session)
		}
		// Ignore errors for individual sessions (they might have expired)
	}

	return sessions, nil
}

func (r *presenceRepo) DisconnectDeviceSession(ctx context.Context, clientID string) error {
	session, err := r.GetDeviceSession(ctx, clientID)
	if err != nil {
		return err
	}

	now := time.Now()
	session.DisconnectedAt = &now

	sessionKey := fmt.Sprintf("%s%s", deviceSessionPrefix, clientID)
	userSessionsKey := fmt.Sprintf("%s%s", userSessionsPrefix, session.UserID.String())

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	pipe := r.redis.Pipeline()
	pipe.Set(ctx, sessionKey, data, time.Hour) // Keep disconnected sessions for 1 hour
	pipe.SRem(ctx, userSessionsKey, clientID)

	_, err = pipe.Exec(ctx)
	return err
}

func (r *presenceRepo) GetStaleDeviceSessions(ctx context.Context, timeout time.Duration) ([]*biz.DeviceSession, error) {
	// This is a simplified implementation
	// In a real system, you might want to use a more sophisticated approach
	// like scanning through all session keys or maintaining a separate index
	
	cutoff := time.Now().Add(-timeout)
	staleSessions := []*biz.DeviceSession{}

	// Scan for all device session keys
	iter := r.redis.Scan(ctx, 0, deviceSessionPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		clientID := key[len(deviceSessionPrefix):]
		
		session, err := r.GetDeviceSession(ctx, clientID)
		if err != nil {
			continue
		}

		if session.DisconnectedAt == nil && session.LastHeartbeat.Before(cutoff) {
			staleSessions = append(staleSessions, session)
		}
	}

	return staleSessions, iter.Err()
}

func (r *presenceRepo) CleanupStalePresence(ctx context.Context, timeout time.Duration) error {
	// Clean up expired sessions and update user presence accordingly
	cutoff := time.Now().Add(-timeout)

	// Scan for all user presence keys
	iter := r.redis.Scan(ctx, 0, userPresencePrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		userIDStr := key[len(userPresencePrefix):]
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			continue
		}

		presence, err := r.GetUserPresence(ctx, userID)
		if err != nil {
			continue
		}

		// If user hasn't been seen recently and has no active sessions, mark as offline
		if presence.LastSeen.Before(cutoff) {
			sessions, err := r.GetUserDeviceSessions(ctx, userID)
			if err != nil {
				continue
			}

			hasActiveSessions := false
			for _, session := range sessions {
				if session.DisconnectedAt == nil && session.LastHeartbeat.After(cutoff) {
					hasActiveSessions = true
					break
				}
			}

			if !hasActiveSessions && presence.Status != biz.StatusOffline {
				presence.Status = biz.StatusOffline
				r.SetUserPresence(ctx, presence)
			}
		}
	}

	return iter.Err()
}
