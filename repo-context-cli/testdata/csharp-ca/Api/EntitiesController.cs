using Fixture.Application;

namespace Fixture.Api;

public class EntitiesController
{
    private readonly IEntityRepository repo;

    public EntitiesController(IEntityRepository repo)
    {
        this.repo = repo;
    }

    public object Create(string name) => new CreateEntity(repo).Execute(name);

    [HttpPost("entities")]
    public object CreateHttp(string name) => Create(name);
}
