using Fixture.Api;
using Fixture.Infrastructure;

namespace Fixture.Composition;

public static class Program
{
    public static object Run(string name) =>
        new EntitiesController(new Repo()).Create(name);
}
