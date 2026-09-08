namespace Fixture.Domain;

public interface IEntity
{
    string Id { get; }
}

public class Entity : IEntity
{
    public string Id { get; set; } = "";
}
