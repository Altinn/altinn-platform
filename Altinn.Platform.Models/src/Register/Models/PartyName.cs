using System.Text.Json.Serialization;

namespace Altinn.Platform.Register.Models
{
    /// <summary>
    /// Represents the party lookup result
    /// </summary>
    public class PartyName
    {
        /// <summary>
        /// Gets or sets the social security number for this result.
        /// </summary>
        [JsonPropertyName("ssn")]
        public string Ssn { get; set; }

        /// <summary>
        /// Gets or sets the organization number for this result.
        /// </summary>
        [JsonPropertyName("orgNo")]
        public string OrgNo { get; set; }

        /// <summary>
        /// Gets or sets the party name for this result
        /// </summary>
        [JsonPropertyName("name")]
        public string Name { get; set; }
    }
}
