using Fixture.Domain;

namespace Fixture.Application;

public class CreateEntity
{
    private readonly IEntityRepository repo;

    public CreateEntity(IEntityRepository repo)
    {
        this.repo = repo;
    }

    public Entity Execute(string name)
    {
        var e = new Entity { Id = "1" };
        repo.Save(e);
        return e;
    }
}
