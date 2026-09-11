package nodehttp

import (
	"strings"
	"testing"
)

const buildApp = `
await app.register(meTimeRoutes, {
  prefix: "/api/me/time",
  container,
});
await app.register(scheduleRoutes, { prefix: "/api/schedules", container });
`

const meTime = `
import type { FastifyPluginAsync } from "fastify";

export const meTimeRoutes: FastifyPluginAsync<{ container: AppContainer }> = async (app, opts) => {
  const { container } = opts;
  app.post("/clock-in", { preHandler: [authenticate] }, async (req) => {
    const result = await container.clockIn.execute({
      userId: userId(req.auth.sub),
    });
    return result;
  });
  app.get("/status", async () => container.getTimeStatus.execute());
};
`

const schedules = `
export async function scheduleRoutes(app, opts) {
  const { container } = opts;
  app.get("/", { preHandler: [authenticate] }, async () => {
    return container.listSchedules.execute();
  });
  app.patch("/:id", async (req) => container.updateSchedule.execute(req.params));
};
`

const container = `
export interface AppContainer {
  users: UserRepository;
  clockIn: ClockInUseCase;
  getTimeStatus: GetTimeStatusUseCase;
  listSchedules: ListSchedulesUseCase;
  updateSchedule: UpdateScheduleUseCase;
}

export function createContainer(env: AppEnv): AppContainer {
  const users = new SqliteUserRepository(db);
  const clockIn = new ClockInUseCase(timeSessions, shifts, clock);
  const getTimeStatus = new GetTimeStatusUseCase(timeSessions);
  const listSchedules = new ListSchedulesUseCase(schedules);
  const updateSchedule = new UpdateScheduleUseCase(schedules);
  return { users, clockIn, getTimeStatus, listSchedules, updateSchedule };
}
`

func TestFlushFastifyPrefixAndCompositionBinds(t *testing.T) {
	a := New()
	a.Scan("apps/schedule-api/src/http/build-app.ts", buildApp)
	a.Scan("apps/schedule-api/src/http/routes/me-time.ts", meTime)
	a.Scan("apps/schedule-api/src/http/routes/schedules.ts", schedules)
	a.Scan("apps/schedule-api/src/composition/container.ts", container)
	eps, dis := Flush(a)

	wantRoute := "POST /api/me/time/clock-in"
	got := ""
	handler := ""
	auth := ""
	for _, e := range eps {
		key := e.Method + " " + e.Route
		if key == wantRoute {
			got = key
			handler = e.Handler
			auth = e.Auth
		}
	}
	if got == "" {
		t.Fatalf("missing %s in %#v", wantRoute, eps)
	}
	if handler != "ClockInUseCase" {
		t.Fatalf("handler=%s", handler)
	}
	if auth != "preHandler" {
		t.Fatalf("auth=%s", auth)
	}

	var patch bool
	for _, e := range eps {
		if e.Method == "PATCH" && e.Route == "/api/schedules/:id" {
			patch = true
			if e.Handler != "UpdateScheduleUseCase" {
				t.Fatalf("patch handler=%s", e.Handler)
			}
		}
	}
	if !patch {
		t.Fatal("expected PATCH /api/schedules/:id")
	}

	bind := false
	for _, d := range dis {
		if d.Interface == "UserRepository" && d.Implementation == "SqliteUserRepository" {
			bind = true
			if d.Lifetime != "singleton" {
				t.Fatalf("lifetime=%s", d.Lifetime)
			}
		}
	}
	if !bind {
		t.Fatalf("missing UserRepository bind in %#v", dis)
	}
}

func TestJoinRoute(t *testing.T) {
	if g := joinRoute("/api/me/time", "/clock-in"); g != "/api/me/time/clock-in" {
		t.Fatal(g)
	}
	if g := joinRoute("/api/schedules", "/"); g != "/api/schedules" {
		t.Fatal(g)
	}
	if g := joinRoute("", "clock-in"); !strings.HasPrefix(g, "/") {
		t.Fatal(g)
	}
}
