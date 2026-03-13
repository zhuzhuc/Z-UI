package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	statsService "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type InboundTrafficStat struct {
	Tag      string `json:"tag"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
	Total    int64  `json:"total"`
}

type UserTrafficStat struct {
	User     string `json:"user"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
	Total    int64  `json:"total"`
}

type XrayStatsOverview struct {
	At             time.Time            `json:"at"`
	InboundTraffic []InboundTrafficStat `json:"inboundTraffic"`
	UserTraffic    []UserTrafficStat    `json:"userTraffic"`
	InboundTotal   int64                `json:"inboundTotal"`
	UserTotal      int64                `json:"userTotal"`
}

func (m *XrayManager) QueryStatsOverview() (XrayStatsOverview, error) {
	conn, err := grpc.Dial(m.apiAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return XrayStatsOverview{}, err
	}
	defer conn.Close()

	client := statsService.NewStatsServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{
		Pattern: "",
		Reset_:  false,
	})
	if err != nil {
		return XrayStatsOverview{}, err
	}

	inboundMap := map[string]*InboundTrafficStat{}
	userMap := map[string]*UserTrafficStat{}
	var inboundTotal int64
	var userTotal int64

	for _, st := range resp.Stat {
		name := st.Name
		value := st.Value

		if strings.HasPrefix(name, "inbound>>>") && strings.Contains(name, ">>>traffic>>>") {
			tag, typ, ok := parseInboundStatName(name)
			if !ok {
				continue
			}
			entry := inboundMap[tag]
			if entry == nil {
				entry = &InboundTrafficStat{Tag: tag}
				inboundMap[tag] = entry
			}
			if typ == "uplink" {
				entry.Uplink += value
			}
			if typ == "downlink" {
				entry.Downlink += value
			}
			entry.Total = entry.Uplink + entry.Downlink
			inboundTotal += value
			continue
		}

		if strings.HasPrefix(name, "user>>>") && strings.Contains(name, ">>>traffic>>>") {
			user, typ, ok := parseUserStatName(name)
			if !ok {
				continue
			}
			entry := userMap[user]
			if entry == nil {
				entry = &UserTrafficStat{User: user}
				userMap[user] = entry
			}
			if typ == "uplink" {
				entry.Uplink += value
			}
			if typ == "downlink" {
				entry.Downlink += value
			}
			entry.Total = entry.Uplink + entry.Downlink
			userTotal += value
		}
	}

	inboundTraffic := make([]InboundTrafficStat, 0, len(inboundMap))
	for _, v := range inboundMap {
		inboundTraffic = append(inboundTraffic, *v)
	}
	sort.Slice(inboundTraffic, func(i, j int) bool { return inboundTraffic[i].Tag < inboundTraffic[j].Tag })

	userTraffic := make([]UserTrafficStat, 0, len(userMap))
	for _, v := range userMap {
		userTraffic = append(userTraffic, *v)
	}
	sort.Slice(userTraffic, func(i, j int) bool { return userTraffic[i].User < userTraffic[j].User })

	return XrayStatsOverview{
		At:             time.Now(),
		InboundTraffic: inboundTraffic,
		UserTraffic:    userTraffic,
		InboundTotal:   inboundTotal,
		UserTotal:      userTotal,
	}, nil
}

func parseInboundStatName(name string) (tag string, typ string, ok bool) {
	parts := strings.Split(name, ">>>")
	// inbound>>>tag>>>traffic>>>uplink
	if len(parts) != 4 || parts[0] != "inbound" || parts[2] != "traffic" {
		return "", "", false
	}
	return parts[1], parts[3], true
}

func parseUserStatName(name string) (user string, typ string, ok bool) {
	parts := strings.Split(name, ">>>")
	// user>>>email>>>traffic>>>uplink
	if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
		return "", "", false
	}
	return parts[1], parts[3], true
}

func bytesToGB(v int64) int64 {
	if v <= 0 {
		return 0
	}
	const gb = int64(1024 * 1024 * 1024)
	if v%gb == 0 {
		return v / gb
	}
	return (v / gb) + 1
}

func (m *XrayManager) SyncInboundUsageFromStats() (int, error) {
	overview, err := m.QueryStatsOverview()
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, item := range overview.InboundTraffic {
		usedGB := bytesToGB(item.Total)
		if err := m.store.UpdateInboundUsedGBByTag(item.Tag, usedGB); err != nil {
			return updated, fmt.Errorf("update inbound %s used_gb failed: %w", item.Tag, err)
		}
		updated++
	}
	return updated, nil
}
