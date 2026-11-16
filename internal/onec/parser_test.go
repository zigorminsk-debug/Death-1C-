package onec

import "testing"

func TestParseClustersInfobasesAndSessions(t *testing.T) {
	clusterOutput := `
cluster                       : 35629a22-5023-11eb-9415-00155d012a05
host                          : srv-1c
port                          : 1541
name                          : "Локальный кластер"
expiration-timeout            : 0
`
	clusters := ParseClusters(clusterOutput)
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].ID != "35629a22-5023-11eb-9415-00155d012a05" || clusters[0].Name != "Локальный кластер" || clusters[0].Port != 1541 {
		t.Fatalf("unexpected cluster parsed: %+v", clusters[0])
	}

	ibOutput := `
infobase : 7c8b9d00-5023-11eb-9415-00155d012a05
name     : Buh_Prod
descr    : "Бухгалтерия для Беларуси"

infobase : 8f112233-5023-11eb-9415-00155d012a05
name     : ZUP_Prod
descr    : "Зарплата и управление персоналом"
`
	ibs := ParseInfobases(ibOutput)
	if len(ibs) != 2 {
		t.Fatalf("expected 2 infobases, got %d", len(ibs))
	}

	sessionOutput := `
session                          : a1b2c3d4-e5f6-7890-abcd-ef1234567890
session-id                       : 145
infobase                         : 7c8b9d00-5023-11eb-9415-00155d012a05
user-name                        : "Захаревич И."
host                             : WS-ADMIN-01
client-ip                        : 192.168.10.25
app-id                           : 1CV8C
started-at                       : 2026-11-16T08:15:23
last-active-at                   : 2026-11-16T10:42:11
hibernate                        : no

session                          : ffeeddcc-bbaa-9988-7766-554433221100
session-id                       : 112
infobase                         : 7c8b9d00-5023-11eb-9415-00155d012a05
user-name                        : "Захаревич И."
host                             : WS-ADMIN-01
app-id                           : 1CV8C
started-at                       : 2026-11-16T07:30:00
last-active-at                   : 2026-11-16T08:10:05
hibernate                        : yes
`
	sessions := ParseSessions(sessionOutput, clusters[0], ibs)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].UserName != "Захаревич И." {
		t.Fatalf("unexpected UserName: %q", sessions[0].UserName)
	}
	if sessions[0].InfobaseName != "Buh_Prod" || sessions[0].InfobaseDescr != "Бухгалтерия для Беларуси" {
		t.Fatalf("infobase metadata not linked: %+v", sessions[0])
	}
	if sessions[0].Hibernate || !sessions[1].Hibernate {
		t.Fatalf("hibernate flags parsed inaccurately")
	}
	if sessions[0].StartedAt.IsZero() || sessions[0].LastActiveAt.IsZero() {
		t.Fatalf("timestamps failed to parse: %+v", sessions[0])
	}
}
