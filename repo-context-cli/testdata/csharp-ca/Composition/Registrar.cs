using Fixture.Application;
using Fixture.Infrastructure;

namespace Fixture.Composition;

public static class Registrar
{
    public static void Register(object services)
    {
        services.AddScoped<IEntityRepository, Repo>();
    }
}
