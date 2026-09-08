using Fixture.Domain;
using Fixture.Infrastructure;

namespace Fixture.Application;

public class CreateEntity
{
    public Entity Execute(string name)
    {
        var e = new Entity { Id = "1" };
        Repo.Save(e);
        return e;
    }
}
