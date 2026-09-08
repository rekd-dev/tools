using Fixture.Domain;
using Fixture.Application;

namespace Fixture.Infrastructure;

public static class Repo
{
    public static void Save(Entity e)
    {
        // Planted cycle: Infrastructure -> Application
        _ = new CreateEntity().Execute(e.Id);
    }
}
