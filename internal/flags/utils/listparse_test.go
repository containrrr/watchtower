package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSplitNotificationValuesComprehensive exercises SplitNotificationValues for Shoutrrr
// URL lists with separators and commas inside query/tenant segments.
func TestSplitNotificationValuesComprehensive(t *testing.T) {
	testCases := []struct {
		name     string
		envValue string
		expected []string
	}{
		// SMTP service tests
		{
			name:     "SMTP single URL",
			envValue: "smtp://user:pass@host:port/?from=test@example.com&to=recipient@example.com",
			expected: []string{
				"smtp://user:pass@host:port/?from=test@example.com&to=recipient@example.com",
			},
		},
		{
			name:     "SMTP multiple recipients with comma in query",
			envValue: "smtp://user:pass@host:port/?from=test@example.com&to=recipient1@example.com,recipient2@example.com",
			expected: []string{
				"smtp://user:pass@host:port/?from=test@example.com&to=recipient1@example.com,recipient2@example.com",
			},
		},
		{
			name:     "SMTP space separator",
			envValue: "smtp://user:pass@host1:port smtp://user:pass@host2:port",
			expected: []string{"smtp://user:pass@host1:port", "smtp://user:pass@host2:port"},
		},
		{
			name:     "SMTP comma-space separator",
			envValue: "smtp://user:pass@host1:port, smtp://user:pass@host2:port",
			expected: []string{"smtp://user:pass@host1:port", "smtp://user:pass@host2:port"},
		},

		// Slack service tests
		{
			name:     "Slack single URL",
			envValue: "slack://botname@token-a/token-b/token-c",
			expected: []string{"slack://botname@token-a/token-b/token-c"},
		},
		{
			name:     "Slack space separator",
			envValue: "slack://token1@channel1 slack://token2@channel2",
			expected: []string{"slack://token1@channel1", "slack://token2@channel2"},
		},
		{
			name:     "Slack comma-space separator",
			envValue: "slack://token1@channel1, slack://token2@channel2",
			expected: []string{"slack://token1@channel1", "slack://token2@channel2"},
		},

		// Gotify service tests
		{
			name:     "Gotify single URL",
			envValue: "gotify://gotify-host/token",
			expected: []string{"gotify://gotify-host/token"},
		},
		{
			name:     "Gotify space separator",
			envValue: "gotify://host1/token1 gotify://host2/token2",
			expected: []string{"gotify://host1/token1", "gotify://host2/token2"},
		},
		{
			name:     "Gotify comma-space separator",
			envValue: "gotify://host1/token1, gotify://host2/token2",
			expected: []string{"gotify://host1/token1", "gotify://host2/token2"},
		},

		// Discord service tests
		{
			name:     "Discord single URL",
			envValue: "discord://token@123456789",
			expected: []string{"discord://token@123456789"},
		},
		{
			name:     "Discord space separator",
			envValue: "discord://token1@123 discord://token2@456",
			expected: []string{"discord://token1@123", "discord://token2@456"},
		},
		{
			name:     "Discord comma-space separator",
			envValue: "discord://token1@123, discord://token2@456",
			expected: []string{"discord://token1@123", "discord://token2@456"},
		},

		// Teams service tests
		{
			name:     "Teams single URL",
			envValue: "teams://group@tenant/altId/groupOwner?host=organization.webhook.office.com",
			expected: []string{
				"teams://group@tenant/altId/groupOwner?host=organization.webhook.office.com",
			},
		},
		{
			name:     "Teams space separator",
			envValue: "teams://group1@tenant1/id1/owner1?host=host1 teams://group2@tenant2/id2/owner2?host=host2",
			expected: []string{
				"teams://group1@tenant1/id1/owner1?host=host1",
				"teams://group2@tenant2/id2/owner2?host=host2",
			},
		},
		{
			name:     "Teams comma-space separator",
			envValue: "teams://group1@tenant1/id1/owner1?host=host1, teams://group2@tenant2/id2/owner2?host=host2",
			expected: []string{
				"teams://group1@tenant1/id1/owner1?host=host1",
				"teams://group2@tenant2/id2/owner2?host=host2",
			},
		},

		// Telegram service tests
		{
			name:     "Telegram single URL",
			envValue: "telegram://1234567890:AAEJ_AAAAABBBBBccccccccdddddddd@telegram/?channels=123456789&parseMode=html",
			expected: []string{
				"telegram://1234567890:AAEJ_AAAAABBBBBccccccccdddddddd@telegram/?channels=123456789&parseMode=html",
			},
		},
		{
			name:     "Telegram space separator",
			envValue: "telegram://1234567890:AAEJ_AAAAABBBBBccccccccdddddddd@telegram/?channels=123456789&parseMode=html telegram://another@telegram",
			expected: []string{
				"telegram://1234567890:AAEJ_AAAAABBBBBccccccccdddddddd@telegram/?channels=123456789&parseMode=html",
				"telegram://another@telegram",
			},
		},
		{
			name:     "Telegram comma-space separator",
			envValue: "telegram://1234567890:AAEJ_AAAAABBBBBccccccccdddddddd@telegram/?channels=123456789&parseMode=html, telegram://another@telegram",
			expected: []string{
				"telegram://1234567890:AAEJ_AAAAABBBBBccccccccdddddddd@telegram/?channels=123456789&parseMode=html",
				"telegram://another@telegram",
			},
		},

		// Generic webhook tests
		{
			name:     "Generic webhook single URL",
			envValue: "generic+https://webhook.example.com/hook?token=abc123",
			expected: []string{"generic+https://webhook.example.com/hook?token=abc123"},
		},
		{
			name:     "Generic webhook space separator",
			envValue: "generic+https://hook1.example.com generic+https://hook2.example.com",
			expected: []string{
				"generic+https://hook1.example.com",
				"generic+https://hook2.example.com",
			},
		},
		{
			name:     "Generic webhook comma-space separator",
			envValue: "generic+https://hook1.example.com, generic+https://hook2.example.com",
			expected: []string{
				"generic+https://hook1.example.com",
				"generic+https://hook2.example.com",
			},
		},

		// Edge cases
		{
			name:     "URL with comma in query parameter",
			envValue: "https://api.example.com/webhook?param=value,with,commas https://api2.example.com/webhook",
			expected: []string{
				"https://api.example.com/webhook?param=value,with,commas",
				"https://api2.example.com/webhook",
			},
		},
		{
			name:     "Multiple URLs with mixed separators",
			envValue: "smtp://test1 smtp://test2 slack://test3 slack://test4 gotify://test5",
			expected: []string{
				"smtp://test1",
				"smtp://test2",
				"slack://test3",
				"slack://test4",
				"gotify://test5",
			},
		},
		{
			name:     "Empty values filtered out",
			envValue: "smtp://test1 smtp://test2 slack://test3",
			expected: []string{"smtp://test1", "smtp://test2", "slack://test3"},
		},
		{
			name:     "Malformed URL handling",
			envValue: "not-a-url smtp://valid@example.com invalid://missing-parts",
			expected: []string{"not-a-url", "smtp://valid@example.com", "invalid://missing-parts"},
		},
		{
			name:     "URLs with special characters",
			envValue: "smtp://user%40domain:pass%40word@host:587 slack://token@channel",
			expected: []string{
				"smtp://user%40domain:pass%40word@host:587",
				"slack://token@channel",
			},
		},
		{
			name: "Very long URL",
			envValue: "https://very-long-domain-name-with-many-subdomains.example.com/path/to/webhook?param1=" + strings.Repeat(
				"a",
				1000,
			),
			expected: []string{
				"https://very-long-domain-name-with-many-subdomains.example.com/path/to/webhook?param1=" + strings.Repeat(
					"a",
					1000,
				),
			},
		},
		// Test cases from issues and bug reports
		{
			name:     "URL with comma in query parameter should not be split",
			envValue: "smtp://user:pass@host:port/?to=recipient1@example.com,recipient2@example.com",
			expected: []string{
				"smtp://user:pass@host:port/?to=recipient1@example.com,recipient2@example.com",
			},
		},
		{
			name:     "Multiple URLs with comma in second URL query",
			envValue: "smtp://test1 smtp://test2?param=value,with,commas",
			expected: []string{"smtp://test1", "smtp://test2?param=value,with,commas"},
		},
		{
			name:     "Complex URL with commas in path and query",
			envValue: "https://api.example.com/webhook?param=value,with,commas https://api2.example.com/webhook",
			expected: []string{
				"https://api.example.com/webhook?param=value,with,commas",
				"https://api2.example.com/webhook",
			},
		},
		{
			name:     "Teams URL with comma in tenant ID",
			envValue: "teams://group@tenant,id.with,commas/altId/groupOwner?host=organization.webhook.office.com",
			expected: []string{
				"teams://group@tenant,id.with,commas/altId/groupOwner?host=organization.webhook.office.com",
			},
		},

		// Additional edge cases
		{
			name:     "URL with multiple commas in query parameters",
			envValue: "smtp://user:pass@host:port/?to=recipient1@example.com,recipient2@example.com,recipient3@example.com",
			expected: []string{
				"smtp://user:pass@host:port/?to=recipient1@example.com,recipient2@example.com,recipient3@example.com",
			},
		},
		{
			name:     "URL with encoded commas",
			envValue: "smtp://user:pass@host:port/?to=recipient1%2Crecipient2%2Crecipient3@example.com",
			expected: []string{
				"smtp://user:pass@host:port/?to=recipient1%2Crecipient2%2Crecipient3@example.com",
			},
		},
		{
			name:     "IPv6 address in URL",
			envValue: "smtp://[::1]:587/?from=test@example.com",
			expected: []string{
				"smtp://[::1]:587/?from=test@example.com",
			},
		},
		{
			name:     "Complex authentication with encoded characters",
			envValue: "smtp://user%40domain:pass%40word@host:587",
			expected: []string{
				"smtp://user%40domain:pass%40word@host:587",
			},
		},
		{
			name:     "URL with special characters in query",
			envValue: "generic+https://api.example.com/webhook?param=value&special=!@#$%^&*()",
			expected: []string{
				"generic+https://api.example.com/webhook?param=value&special=!@#$%^&*()",
			},
		},
		{
			name:     "Multiple URLs with commas in different positions",
			envValue: "smtp://test1 smtp://test2?param=value,with,commas gotify://host/token",
			expected: []string{
				"smtp://test1",
				"smtp://test2?param=value,with,commas",
				"gotify://host/token",
			},
		},
		{
			name:     "Multiple IPv6 URLs",
			envValue: "smtp://[::1]:587 smtp://[2001:db8::1]:587",
			expected: []string{
				"smtp://[::1]:587",
				"smtp://[2001:db8::1]:587",
			},
		},
		{
			name:     "URL with encoded special characters in path",
			envValue: "slack://token@channel?text=Hello%20World%21",
			expected: []string{
				"slack://token@channel?text=Hello%20World%21",
			},
		},
		{
			name:     "Teams URL with complex tenant and commas",
			envValue: "teams://group@tenant.with.dots.and,commas,more/altId/groupOwner?host=organization.webhook.office.com",
			expected: []string{
				"teams://group@tenant.with.dots.and,commas,more/altId/groupOwner?host=organization.webhook.office.com",
			},
		},
		{
			name: "Very long URL with multiple commas",
			envValue: "https://very-long-domain-name-with-many-subdomains.example.com/path/to/webhook?param1=" + strings.Repeat(
				"a,b,c,",
				50,
			) + "end",
			expected: []string{
				"https://very-long-domain-name-with-many-subdomains.example.com/path/to/webhook?param1=" + strings.Repeat(
					"a,b,c,",
					50,
				) + "end",
			},
		},
		{
			name:     "URL with authentication and IPv6",
			envValue: "smtp://user:pass@[2001:db8::1]:587/?from=test@example.com",
			expected: []string{
				"smtp://user:pass@[2001:db8::1]:587/?from=test@example.com",
			},
		},
		{
			name:     "Mixed separators with complex URLs",
			envValue: "smtp://test1, smtp://test2?param=value,with,commas gotify://host/token slack://token@channel",
			expected: []string{
				"smtp://test1",
				"smtp://test2?param=value,with,commas",
				"gotify://host/token",
				"slack://token@channel",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			urls := FilterEmptyStrings(SplitNotificationValues(tc.envValue))
			assert.Equal(t, tc.expected, urls)
		})
	}
}

// TestFilterEmptyStrings verifies FilterEmptyStrings function.
func TestFilterEmptyStrings(t *testing.T) {
	tests := []struct {
		input    []string
		expected any
	}{
		{[]string{"a", "", "b"}, []string{"a", "b"}},
		{[]string{"  ", "c", "\t"}, []string{"c"}},
		{[]string{}, nil},
		{[]string{"", " ", ""}, nil},
		{[]string{"valid"}, []string{"valid"}},
	}

	for _, tt := range tests {
		result := FilterEmptyStrings(tt.input)
		if tt.expected == nil {
			assert.Nil(t, result)
		} else {
			assert.Equal(t, tt.expected, result)
		}
	}
}
