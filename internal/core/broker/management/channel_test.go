package management

import (
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enhancedFakeBroker extends fakeBroker with channel tracking
type enhancedFakeBroker struct {
	*fakeBroker
	channels           []models.ChannelInfo
	connectionChannels map[string][]models.ChannelInfo
}

func (efb *enhancedFakeBroker) ListChannels(vhost string) ([]models.ChannelInfo, error) {
	if vhost == "" {
		return efb.channels, nil
	}
	// Filter by vhost
	filtered := []models.ChannelInfo{}
	for _, ch := range efb.channels {
		if ch.VHost == vhost {
			filtered = append(filtered, ch)
		}
	}
	return filtered, nil
}

func (efb *enhancedFakeBroker) ListConnectionChannels(connectionName string) ([]models.ChannelInfo, error) {
	if channels, ok := efb.connectionChannels[connectionName]; ok {
		return channels, nil
	}
	return []models.ChannelInfo{}, nil
}

func (efb *enhancedFakeBroker) CreateChannelInfo(connID vhost.ConnectionID, channelNum uint16, vh *vhost.VHost) (models.ChannelInfo, error) {
	return models.ChannelInfo{
		Number:         channelNum,
		ConnectionName: string(connID),
		VHost:          vh.Name,
		State:          "running",
		UnackedCount:   0,
		PrefetchCount:  0,
	}, nil
}

func setupTestBrokerWithChannels(t *testing.T) *enhancedFakeBroker {
	t.Helper()
	base := setupTestBroker(t).(*fakeBroker)

	channels := []models.ChannelInfo{
		{
			Number:         1,
			ConnectionName: "conn1",
			VHost:          "/",
			State:          "running",
			UnackedCount:   5,
			PrefetchCount:  10,
		},
		{
			Number:         2,
			ConnectionName: "conn1",
			VHost:          "/",
			State:          "running",
			UnackedCount:   0,
			PrefetchCount:  0,
		},
		{
			Number:         1,
			ConnectionName: "conn2",
			VHost:          "/",
			State:          "flow",
			UnackedCount:   15,
			PrefetchCount:  20,
		},
	}

	connChannels := map[string][]models.ChannelInfo{
		"conn1": {channels[0], channels[1]},
		"conn2": {channels[2]},
	}

	return &enhancedFakeBroker{
		fakeBroker:         base,
		channels:           channels,
		connectionChannels: connChannels,
	}
}

func TestListChannels_AllVHosts(t *testing.T) {
	broker := setupTestBrokerWithChannels(t)
	service := NewService(broker)

	channels, err := service.ListChannels("")
	require.NoError(t, err)

	assert.Len(t, channels, 3)
	assert.Equal(t, uint16(1), channels[0].Number)
	assert.Equal(t, "conn1", channels[0].ConnectionName)
	assert.Equal(t, 5, channels[0].UnackedCount)
	assert.Equal(t, uint16(10), channels[0].PrefetchCount)
}

func TestListChannels_SpecificVHost(t *testing.T) {
	broker := setupTestBrokerWithChannels(t)
	service := NewService(broker)

	channels, err := service.ListChannels("/")
	require.NoError(t, err)

	assert.Len(t, channels, 3)
	for _, ch := range channels {
		assert.Equal(t, "/", ch.VHost)
	}
}

func TestListChannels_NonExistentVHost(t *testing.T) {
	broker := setupTestBrokerWithChannels(t)
	service := NewService(broker)

	_, err := service.ListChannels("/nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListConnectionChannels_Success(t *testing.T) {
	broker := setupTestBrokerWithChannels(t)
	service := NewService(broker)

	channels, err := service.ListConnectionChannels("conn1")
	require.NoError(t, err)

	assert.Len(t, channels, 2)
	assert.Equal(t, "conn1", channels[0].ConnectionName)
	assert.Equal(t, "conn1", channels[1].ConnectionName)
}

func TestListConnectionChannels_NoChannels(t *testing.T) {
	broker := setupTestBrokerWithChannels(t)
	service := NewService(broker)

	channels, err := service.ListConnectionChannels("nonexistent")
	require.NoError(t, err)

	assert.Empty(t, channels)
}

func TestGetChannel_Success(t *testing.T) {
	broker := setupTestBrokerWithChannels(t)
	service := NewService(broker)

	channel, err := service.GetChannel("conn1", 1)
	require.NoError(t, err)
	require.NotNil(t, channel)

	assert.Equal(t, uint16(1), channel.Number)
	assert.Equal(t, "conn1", channel.ConnectionName)
	assert.Equal(t, "/", channel.VHost)
}
