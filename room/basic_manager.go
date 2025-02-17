package room

import (
	"context"
	"sync"
	"time"

	"github.com/HuolalaTech/page-spy-api/api/room"
	"github.com/HuolalaTech/page-spy-api/logger"
	"github.com/HuolalaTech/page-spy-api/metric"
	"github.com/HuolalaTech/page-spy-api/state"
)

var log = logger.Log()

func NewBasicManager() *BasicManager {
	return &BasicManager{
		StatusMachine: *state.NewStatusMachine(),
		roomsMap:      make(map[string]room.ManagerRoom),
	}
}

type BasicManager struct {
	state.StatusMachine
	rwLock   sync.RWMutex
	roomsMap map[string]room.ManagerRoom
}

func (r *BasicManager) addRoom(room room.ManagerRoom) {
	r.rwLock.Lock()
	defer r.rwLock.Unlock()
	r.roomsMap[room.GetRoomAddress().ID] = room
}

func equalTags(old map[string]string, new map[string]string) bool {
	if len(old) <= 0 && len(new) <= 0 {
		return true
	}

	if len(new) <= 0 || len(old) <= 0 {
		return false
	}

	for k, v := range new {
		if old[k] != v {
			return false
		}
	}

	return true
}

func (r *BasicManager) getRoomsByTags(tags map[string]string) []room.ManagerRoom {
	rooms := r.getRooms()
	ret := []room.ManagerRoom{}
	if len(tags) <= 0 {
		return rooms
	}

	for _, rr := range rooms {
		if rr.GetInfo() != nil && len(rr.GetInfo().Tags) > 0 {
			if equalTags(rr.GetInfo().Tags, tags) {
				ret = append(ret, rr)
			}
		}
	}

	return ret
}
func (r *BasicManager) getRooms() []room.ManagerRoom {
	r.rwLock.Lock()
	defer r.rwLock.Unlock()
	ret := []room.ManagerRoom{}
	for _, v := range r.roomsMap {
		ret = append(ret, v)
	}
	return ret
}

func (r *BasicManager) getRoom(opt *room.Info) (room.ManagerRoom, bool) {
	r.rwLock.RLock()
	defer r.rwLock.RUnlock()
	room, ok := r.roomsMap[opt.Address.ID]
	return room, ok
}

func (r *BasicManager) removeRoom(room room.ManagerRoom) {
	r.rwLock.Lock()
	defer r.rwLock.Unlock()
	delete(r.roomsMap, room.GetRoomAddress().ID)
}

func (r *BasicManager) loop() {
	rooms := r.getRooms()
	value := float64(len(rooms))
	metric.Summary("tunnel_room_manager", map[string]string{
		"action": "create",
		"code":   "success",
	}, value)

	for _, room := range rooms {
		if room.ShouldRemove() {
			r.removeRoom(room)
			err := room.Close(context.Background())
			if err != nil {
				log.WithError(err).Error("loop close room error")
			}
		}
	}
}

func (r *BasicManager) start() {
	if r.IsStatus(state.RunningStatus) {
		return
	}

	r.SetStatus(state.RunningStatus)
	tinker := time.NewTicker(10 * time.Second)
	go func() {
		for range tinker.C {
			r.loop()
		}
	}()
}
