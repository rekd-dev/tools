using Fixture.Domain;

namespace Fixture.Application;

public interface IEntityRepository
{
    void Save(Entity e);
}
