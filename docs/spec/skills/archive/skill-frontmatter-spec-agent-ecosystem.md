# SKILL.md Frontmatter Specification

## Frontmatter fields

All under `metadata`

### top level

-   `skill_metadata_schema_version` Indicates what fields we
    should expect to find in the frontmatter and what those fields
    indicate. Value is a Semver string that does not start with a
    `v` Initial value will be `1.0.0` Note that
    this field represents the version of the schema you\'re currently
    reading.
-   `skill_metadata_schema_definition` URL where the
    definition of the specified version of the schema can be found.

1.  provenance

    (n. the place of origin or earliest known history of something)

    -   `version` Version of the specific skill. If there are
        multiple skills in the repository this may not correspond to the
        release version in the repository. Value is a
        [Semver](https://semver.org/) string Must NOT start with a
        `v` Ex. `4.2.0` not `v4.2.0`

    -   `source_repo` A URL indicating where the latest
        version of this skill can be found. Ex.
        `https://example.com/my_username/cool_skill_repo`

    -   `source_repo_subdirectory` Optional. Only necessary
        when there is more than one skill within the repo. When there is
        only one skill it is assumed to be the repository root. If you
        want to specify this explicitly use `/`

        Given the following Repository file structure the
        \"integration\" skill would have a
        `source_repo_subdirectory` value of
        `/skills/tool_x/integration`

            Repository
            └── skills
                └── tool_x
                    ├── integration
                    └── search

    -   `authors` Optional. A list of strings. Uses the same
        format as git. Typically `{NAME} <{EMAIL}>` Ex.
        `Mary Smith <mary@example.com>`

        Single author:

        ``` yaml
        authors:
          - "Mary Smith <mary@example.com>"
        ```

        Multiple authors:

        ``` yaml
        authors:
          - "Mary Smith <mary@example.com>"
          - "Andrea Barley"
        ```

    -   `license_name` Optional but *strongly encouraged.* A
        short human readable name for the license. Ex. `MIT`
        or `GPL v3.0`

    -   `license_url` Optional but *strongly encouraged.* The
        URL to the full license including author and copyright
        attribution. Open Source licenses are built on the foundation of
        copyright law. The license name is useless without knowledge of
        who holds the copyright or the specific wording used in their
        copy of the specified license.

        Note that authors are not necessarily the copyright holders.
        Most work created during your day job will actually belong to
        the company you created it for.

        Example for the \"integration\" skill in the directory structure
        example above.

        ``` yaml
        metadata:
          skill_metadata_schema_version: 1.0.0
          skill_metadata_schema_definition: https://example.com/skill_metadata_schema/v1_0_0
          provenance:
            version: 4.2.0
            source_repo: https://example.com/my_username/cool_skill_repo
            source_repo_subdirectory: /skills/tool_x/integration
            authors:
              - "Mary Smith <mary@example.com>"
            license_name: MIT
            license_url: https://example.com/my_username/cool_skill_repo/LICENSE.md
        ```

2.  expectations

    Whenever specifying versions use the [same version constraint syntax
    as Hex](https://hexdocs.pm/elixir/Version.html). We need to document
    this as part of our spec.

    Full example with all optional expectations items:

    ``` yaml
    metadata:
      skill_metadata_schema_version: 1.0.0
      skill_metadata_schema_definition: https://example.com/skill_metadata_schema/v1_0_0
      expectations:
        software:
          - name: "bat"
            version: ">=0.22"
          - name: "ripgrep"
            version: ">=14.1.0"
        services:
          - name: "Notion"
            url: "https://notion.so"
        programming_environments:
          - name: "ruby"
            version: "~>3.4"
        operating_systems:
          - name: "macOS"
            version: "~>15.7"

    ```

    1.  software

        optional

        An array of strings indicating the apps (typically command line
        apps) that are expected to be present on the system before using
        this skill

        Ex.:

        ``` yaml
        name: "bat"
        version: ">=0.22"
        ```

    2.  services

        optional

        An array of name + url pairs for remote services the user is
        expected to have an account on. This version of the spec does
        attempt to specify how the user will be connecting to or
        interacting with the specified service.

        Ex.:

        ``` yaml
        name: "Notion"
        url: "https://notion.so"
        ```

    3.  programming~environments~

        optional an array of name + version pairs

        Ex.:

        ``` yaml
        name: "ruby"
        version: "~>3.4"
        ```

    4.  operating~systems~

        optional

        an array of name + version pairs the skill is known to work on.
        If not provided it is assumed to be generic and will work on any
        major OS

        Ex.:

        ``` yaml
        name: "macOS"
        version: "~>15.7"
        ```

3.  supported~agentplatforms~

    An array of supported platforms. If not defined it is presumed the
    skill is generic and should work with any platform.

    Many platforms provide integrations to other systems. For example,
    Claude offers \"Google Drive\", and \"GitHub\" \"connectors\". We
    use the term \"integrations\" because it\'s currently vendor neutral
    and does an adequate job of describing what we\'re talking about.

    Each supported agent platform must include a name. It may optionally
    include an array of integrations that must be enabled and configured
    for this particular skill to do its job. If there is a unique
    identifier for the integration it should be used. For example,
    Claude\'s \"Google Drive\" integration has the canonical identifier
    of \"google-drive\". Using a unique canonical identifier helps
    remove ambiguity for the agent reading your `SKILL.md`

    An Integration consists of an `identifier` and a
    `required` field. Non-optional integrations should have
    `required` set to `true` and optional
    integrations should have it set to `false`.

    Ex.:

    ``` yaml
    metadata:
      skill_metadata_schema_version: 1.0.0
      skill_metadata_schema_definition: https://example.com/skill_metadata_schema/v1_0_0
      supported_agent_platforms:
        - name: "Claude Code"
          integrations:
            - identifier: "google-drive"
              required: true
            - identifier: "google-sheets"
              required: false
    ```
