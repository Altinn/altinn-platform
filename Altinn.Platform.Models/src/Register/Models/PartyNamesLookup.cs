using System.Collections.Generic;
using System.Text.Json.Serialization;

namespace Altinn.Platform.Register.Models
{
    /// <summary>
    /// Represents a list of lookup criteria when looking for a Party.
    /// </summary>
    public class PartyNamesLookup
    {
        /// <summary>
        /// Gets or sets the list of identifiers for the parties to look for.
        /// </summary>
        [JsonPropertyName("parties")]
        public List<PartyLookup> Parties { get; set; }
    }
}
