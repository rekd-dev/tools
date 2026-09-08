using Fixture.Application;

namespace Fixture.Api;

public class EntitiesController
{
    public object Create(string name) => new CreateEntity().Execute(name);
}
