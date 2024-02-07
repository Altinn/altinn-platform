using System.Collections.Generic;
using System.Text.Json.Serialization;

namespace Altinn.Platform.Register.Models
{
    /// <summary>
    /// Represents a list of party names for each corresponding identifier
    /// </summary>
    public class PartyNamesLookupResult
    {
        /// <summary>
        /// Gets or sets the list of identifiers for the parties to look for.
        /// </summary>
        [JsonPropertyName("partyNames")]
        public List<PartyName> PartyNames { get; set; }
    }
}
