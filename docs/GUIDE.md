# A Users's Guide to Thunderdome

Thunderdome is a fun way to facilitate agile scrum practices including Story pointing (games), Sprint Retrospectives,
Story mapping, Async Daily Standup (team standup) and more.

As a new user in this realm, let this be your guide. First, we need to know who you are.

## Table of Contents

- [Register](#register-optional)
- [Login](#login)
- [Password Retrieval](#password-retrieval)
- [Profile](#profile)
  - [Details](#details)
  - [API Access](#api-access)
  - [Jira Integration](#jira-integration-premium-only)
  - [Delete Account](#delete-account)
- [Game](#game)
  - [Create a Game](#create-a-game)
  - [Game](#game-1)
  - [Stories](#stories)
  - [Users](#users)
  - [Invite](#invite)
- [Retrospectives](#retrospectives)
  - [Create a Retro](#create-a-retro)
- [Storyboards](#storyboards)
  - [Goals](#goals)
  - [Columns](#columns)
  - [Add Story](#add-story)
  - [Personas](#personas)
  - [Add Persona](#add-persona)
  - [Color Legend](#color-legend)
  - [Edit Legend](#edit-legend)
  - [Create a Storyboard](#create-a-storyboard)
- [Teams, Organizations, and Departments](#teams-organizations-and-departments)
  - [Organizations](#organizations)
  - [Create Organization](#create-organization)
  - [Departments](#departments)
  - [Create Department](#create-department)
  - [Teams](#teams)
  - [Create Team](#create-team)
  - [Add User](#add-user)
  - [Checkins](#checkins)
    - [Check In](#check-in)
    - [Create Games, Retros, and Storyboards](#create-games-retros-and-storyboards)
- [Languages](#languages)
- [Contributions](#contributions)

## Register (optional)

Create a new account, or join as guest.

![Register](img/register.png)

Having an account lets you save your games and more.

![Register Details](img/register-details.png)

- Name  
  This will be visible to others.
- Email (for account)
- Password (for account)  
  Use a strong password. Type it again to confirm.

You will receive an email to confirm your new account.

## Login

Use the email/password you created when registering.

![Login](img/login.png)

- Email
- Password

_OIDC Providers coming soon._

### Password Retrieval

Forgot your password? Thunderdome can send a password reset link to your email.

![Password Retrieval](img/password-retrieval.png)

## Profile

User, it is all about you! Control your Thunderdome experience.

### Details

![Profile Details](img/profile-details.png)

- Name  
  This will be visible to others.
- Email  
  Update your account email, or enter one if you are a guest.
- Country (optional)
- Locale, default: English  
  8 locales to choose from.
- Company (optional)
- Job Title (optional)
- Theme, default: auto  
  The default lets the operating system and browser define dark or light, if supported. If you prefer a darker or
  lighter interface, you may choose it here.
- Option: Enable Game Notification, default: true
- Avatar, default: robohash  
  Several others to choose from, pick your flavor; mp, identicon, monsterid, wavatar, retro.

### API Access

Create an API key to integrate Thunderdome with your tools.

See API Documentation here [Thunderdome API Docs](https://thunderdome.dev/swagger/index.html)

![API Access](img/api-access.png)

![API Key](img/api-key.png)

### Jira Integration (premium only)

Integrate directly with your team's backlog to import your stories to point.

For a private Jira installation, select **Jira Server / Data Center (self-hosted)** and enter its base URL (including `/jira` if your installation uses that path). Choose the authentication method that your Jira supports:

| Installation | Authentication | Account field | Secret field |
| --- | --- | --- | --- |
| Jira Cloud | Email and API token | Atlassian email | Cloud API token |
| Jira Server / Data Center 8.14+ | Personal access token (PAT) | Optional; PAT identifies the user | Jira PAT |
| Older Jira Server, such as 8.3 | Username and password | Jira login username, not necessarily an email | Jira login password |

The private Jira must be reachable from the Thunderdome server. The selected authentication is used for JQL imports, numeric field discovery, and point writeback. Creating or updating a connection automatically verifies its credentials with Jira's read-only current-user API before saving. Failed validation leaves existing settings unchanged and reports authentication, network, certificate, or SSO errors. Checks time out after 10 seconds and do not retry automatically. Existing connections keep their original authentication method. See Atlassian's [PAT version requirements](https://confluence.atlassian.com/enterprise/using-personal-access-tokens-1026032365.html) and [Server basic authentication](https://developer.atlassian.com/server/jira/platform/basic-authentication/).

Use **Test connection** beside a saved Jira instance to recheck it and see the authenticated display name. This checks connectivity and login; permissions to browse or edit individual issues are checked when importing or writing points. No issue is changed by the connection test. The authenticated API equivalent is `POST /api/users/{userId}/jira-instances/{instanceId}/test` (no body), returning `{ "data": { "connected": true, "display_name": "…" } }`. Connection failures return HTTP 422 with a safe, actionable `error`; a Jira 401 does not sign the user out of Thunderdome. Cloud uses [`GET /rest/api/3/myself`](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-myself/); Server / Data Center uses [`GET /rest/api/2/myself`](https://developer.atlassian.com/server/jira/platform/rest/v10000/api-group-myself/).

The connection API also accepts `auth_method: "basic"` or `"pat"`. Set `jira_data_center: true` for Server / Data Center. `client_mail` carries the username for basic authentication and is optional for a PAT; `access_token` carries the selected password or token. Omitting `auth_method` on creation preserves the legacy default (Cloud basic / Data Center PAT), and omitting it on update preserves the saved method.

_Other integrations coming soon._

### Delete Account

This is the Thunderdome, but you are free to leave. We will erase all data about you.

**This is permanent:** All poker sessions, retros, story maps, and orgs/teams directly owned by your account will also
be deleted.

![Delete Account](img/delete-account.png)

## Game

In Thunderdome, an agile poker planning session is known as a Game.

You can create a game to determine the size of a story, or join one in progress.

### Create a Game

![Create Game](img/create-game.png)

- Name
- Team (optional)
- Point Range Allowed: [ 0, 1/2, 1, 2, 3, 5, 8 ]. Higher values and abstention/coffee cards are no longer offered. Historical results and calculated totals are retained.
- Voting countdown: defaults to 2 minutes. A facilitator can set 1–60 whole minutes in **Game Settings**. Changes apply when the next round starts; an active round retains its deadline, including after reload or reconnect.
- Stories  
  Upload an XML or CSV for stories, or add manually. See note in Stories.
- Point Average Rounding, default: Ceil  
  Other options; Round, Floor.
- Option: Auto Finish Voting, default: true
- Option: Hide Voter Identity, default: false
- Passcode (optional)
- Leader Code (optional)

### Game

The planning session is real-time, each user chooses the size for the story and votes are shown when everyone has
finished.

If the team agrees, the game is over. If not, then it has just begun!

![Game Session](img/game-session.png)

#### Choose a voting role

Choose **测试**, **前端开发**, or **后端开发** before voting. Only that role's cards are shown. The browser remembers the choice for this user and game. After casting a ballot, retract it before changing roles; a new round allows another choice. Spectators see the results without voting cards.

When voting ends, everyone sees all three category averages and their total. A category with at least four distinct numeric scores displays **需要讨论**, with its score values. For example, `1/2`, `1`, `3`, and `5` triggers a warning; repeated scores, equivalent values such as `1/2` and `0.5`, nonvoters, and abstentions do not increase the distinct count. Saved results retain the warning. The warning is advisory; Jira writeback is queued only after the facilitator clicks **Save** (保存).

### Stories

This can be a list of Stories, Bugs, Tasks, etc. and serves as a queue for team voting.

#### Import stories from Jira Cloud

Premium feature.

#### Write saved estimates back to Jira

When you create a personal, team, or project poker game, Jira writeback is enabled automatically if your account has exactly one Jira connection with exactly one numeric **Story Points** field (case-insensitive name, ignoring surrounding whitespace). New games without a configured Jira connection keep writeback disabled. Initialization only reads field metadata; Jira points are written only after **Save** (保存). Existing games and manually disabled settings are not changed.

If there are multiple Jira connections, missing or ambiguous fields, connection/permission errors, or settings cannot be saved, the game is still created and a warning explains how to finish setup in **Jira 点数回写**. Initialization is limited to five seconds. Creation on behalf of another user never authorizes that user's credentials; existing subscription requirements still apply.

1. Add a Jira connection under **Profile → Jira Integration** using the authentication method for your installation described above.
2. As a game facilitator, open **Game Settings → Jira 点数回写**.
3. If automatic setup did not apply, enable writeback after **Save** and choose your Jira connection. The site's **Story Points** field is selected automatically when it has a unique match; an existing valid field selection is preserved. The account must be allowed to edit that field on the target issues. If Story Points is missing or ambiguous, check the Jira field configuration or select the intended numeric field before saving.
4. Import stories from Jira, or fill in each story's Jira reference ID and matching issue link. For example, `PROJ-123` and `https://yourjira.atlassian.net/browse/PROJ-123`.

When the configured countdown ends, a facilitator finishes voting, or auto-finish ends the round, the result is displayed without writing to Jira. Click **Save** below the result to save and queue the sum of the testing, frontend, and backend averages for writeback. Only numeric votes count toward each average; nonvoters and historical abstentions do not count. Zero is a valid estimate. Calculated totals can exceed the highest individual card. A round with no numeric votes cannot be saved and does not overwrite Jira. Legacy single-category voting also writes only after the facilitator saves its final numeric estimate.

The result area first prompts the facilitator to click Save, then shows pending, successful, or failed writeback. After Save, the server retries failed requests up to three total attempts, including after a restart. A facilitator can use **重试回写** after fixing account permissions or connection details. The local estimate remains available when Jira is unreachable. Reopening a voting round discards its previous task; changing the destination or disabling writeback cancels affected tasks. Enabling this feature does not send historical estimates. Existing unsaved pending or failed tasks wait for Save after upgrading.

Jira connection choices are private to their owner. The game exposes only the issue key, points, and sync status to participants; credentials stay on the server. The existing subscription requirement for Jira integration applies when subscriptions are enabled.

The same operations are available through the authenticated API:

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/battles/{battleId}/jira-writeback` | Read settings and caller-owned connection choices |
| GET | `/api/battles/{battleId}/jira-writeback/fields?instanceId={id}` | List numeric custom fields |
| PUT | `/api/battles/{battleId}/jira-writeback` | Save `{ "enabled": true, "instanceId": "…", "fieldId": "customfield_…" }` |
| POST | `/api/battles/{battleId}/plans/{planId}/jira-retry` | Retry a failed write for the current round |

All four endpoints require a game facilitator. Writeback uses the official [Jira Cloud issue update API](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/#api-rest-api-3-issue-issueidorkey-put) or [Jira Data Center issue update API](https://developer.atlassian.com/server/jira/platform/updating-an-issue-via-the-jira-rest-apis-6848604/) and changes only the configured field.

#### Import stories from Jira XML

Upload.

#### Import stories from a CSV file

The CSV file must include all the following fields with no header row:

_Type,Title,ReferenceId,Link,Description,AcceptanceCriteria_

#### Create a Story

- Type, default: Story  
  Other Types: Bug, Spike, Epic, Task, Subtask
- Name
- Reference ID
- Link
- Priority  
  Priorities; Blocker, Highest, High, Medium, Low, Lowest
- Description  
  A full text editor is supplied to provide a detailed description.
- Acceptance Criteria  
  A full text editor is supplied, feel free to use Gherkin statements.

### Users

See who is voting or become a spectator.

### Invite

Send a link for others to join the game.

## Retrospectives

Facilitates an agile sprint retrospective.

Retrospectives happen in phases. The first phase is the Prime Directive. You may edit or delete the retro at any time.

1. Prime Directive
2. Brainstorm  
   Add comments. What went well? What needs improvement? I want to ask...
3. Group  
   Organize comments into topics. Drag and drop to sort.
4. Vote  
   Vote for groups to discuss first.
5. Action Items  
   Add Action Items, the grouping and voting phases become locked.
6. Done  
   Export the Retro

![Retrospective](img/retrospective.png)

### Create a Retro

- Name
- Team (optional)
- Join Code (optional)
- Fac. Code (optional)
- Max Group Votes per User, default: 3
- Brainstorm Phase Feedback Visibility, default: Feedback Visible  
  Other options include concealed and hidden. Determines if team members can see each other's suggestions.

## Storyboards

Stories are units of work that need to be sized.

### Goals

A goal is a way to group stories.

- Name (optional)

#### Columns

A column is customizable and serves as a way to track stories throughout the goal.

- Title Text (optional)

#### Add Story

A story is a unit of work. It can be in an open or closed state.

- Name
- Link
- Points
- Color
- Content
- Discussion

### Personas

_Coming Soon_

### Add Persona

- Name
- Role
- Description

### Color Legend

A palette is provided so that you can choose to apply meaningful colors to story cards.

#### Edit Legend

This is where you can define what each color means.

![Color Legend](img/color-legend.png)

### Create a Storyboard

- Name
- Team (optional)
- Passcode (optional)
- Facilitator Code (optional)

## Teams, Organizations, and Departments

![Organizations](img/orgs.png)

Teams can be simple, or they can be within Organizations and Departments.

### Organizations

![Organization](img/organization.png)

#### Create Organization

- Name

### Departments

![Department](img/department.png)

#### Create Department

- Name

### Teams

#### Create Team

- Name

#### Add User

- User Email
- Role  
  Admin or Member

#### Checkins

Asynchronous daily standup tool to aid in speeding up standup or making standups completely async depending on team
practices.

![Checkins](img/checkins.png)

##### Check In

Provide your daily standup report. What did you do yesterday? What are you doing today? Any blockers? Anything to
discuss?

![Checkin Report](img/checkin-report.png)

Choose your timezone.

![Timezone](img/timezone.png)

##### Create Games, Retros, and Storyboards

Creating these sessions within a Team, Organization, or Department will pre-populate those fields.

See each individual section for further details about creating games, retros, and storyboards.

## Languages

🌍 Thunderdome has made every effort to be an international tool, and help developers of all nationalities unite
together.

## Contributions

Thunderdome is released as open source software, the code is hosted on Github and licensed Apache 2.0.

_v3.6.3_
