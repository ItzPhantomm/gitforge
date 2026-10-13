package commands

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gitforge/gitforge/internal/git"
	"github.com/spf13/cobra"
)

// Version holds the current GitForge version.
var Version = "0.1.0-dev"

// NewTUICmd launches the GitForge terminal UI
func NewTUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch the GitForge terminal UI",
		Long:  `Launch the beautiful GitForge terminal UI with interactive commit graph, staging area, and more.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI()
		},
	}
	return cmd
}

// Model represents the TUI state
type Model struct {
	width    int
	height   int
	repo     *git.Repo
	status   git.Status
	branches []git.Branch
	commits  []git.Commit
	remotes  []git.Remote
	focus    int // 0=sidebar, 1=main, 2=detail
	sidebar  SidebarModel
	main     MainModel
	detail   DetailModel
	quitting bool
	err      error
}

// SidebarModel handles the sidebar (repos, branches, remotes)
type SidebarModel struct {
	items    []string
	selected int
	sections []Section
	focused  bool
}

// Section represents a sidebar section
type Section struct {
	name  string
	items []string
	start int
	end   int
}

// MainModel handles the main content area (commit graph, diff)
type MainModel struct {
	content  string
	focused  bool
	viewMode string // "graph", "diff", "status", "log"
}

// DetailModel handles the detail panel (file diff, commit details)
type DetailModel struct {
	content string
	focused bool
}

// Styles
var (
	sidebarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(30)

	mainStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	detailStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(40)

	focusedBorder = lipgloss.NewStyle().
			BorderForeground(lipgloss.Color("205"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

func runTUI() error {
	// Find git repo
	repoPath, err := git.FindRepoRoot(".")
	if err != nil {
		repoPath = "."
	}

	repo, err := git.OpenRepo(repoPath)
	if err != nil {
		// Create a minimal model without repo
		return runTUIWithoutRepo()
	}

	// Load initial data
	status, _ := repo.Status()
	branches, _ := repo.Branches()
	remotes, _ := repo.Remotes()
	commits, _ := repo.Log(&git.LogOptions{Limit: 20})

	m := Model{
		repo:     repo,
		status:   status,
		branches: branches,
		commits:  commits,
		remotes:  remotes,
		focus:    0,
	}

	m.sidebar = NewSidebarModel(repoPath, branches, remotes)
	m.main = NewMainModel(commits)
	m.detail = NewDetailModel()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

func runTUIWithoutRepo() error {
	m := Model{
		focus: 0,
	}
	m.sidebar = NewSidebarModel(".", nil, nil)
	m.main = NewMainModel(nil)
	m.detail = NewDetailModel()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// NewSidebarModel creates a new sidebar model
func NewSidebarModel(repoPath string, branches []git.Branch, remotes []git.Remote) SidebarModel {
	sections := []Section{
		{name: "📁 Repository", items: []string{repoPath}, start: 0, end: 0},
		{name: "🌿 Branches", items: branchNames(branches), start: 0, end: 0},
		{name: "🔗 Remotes", items: remoteNames(remotes), start: 0, end: 0},
		{name: "⚡ Actions", items: []string{"Stage All", "Commit", "Push", "Pull", "Fetch", "Stash"}, start: 0, end: 0},
	}

	var allItems []string
	for i, s := range sections {
		s.start = len(allItems) + i // +i for section headers
		allItems = append(allItems, s.name)
		allItems = append(allItems, s.items...)
		s.end = len(allItems) + i - 1
		sections[i] = s
	}

	return SidebarModel{
		items:    allItems,
		selected: 0,
		sections: sections,
		focused:  true,
	}
}

func branchNames(branches []git.Branch) []string {
	var names []string
	for _, b := range branches {
		prefix := "  "
		if b.IsCurrent {
			prefix = "* "
		}
		names = append(names, prefix+b.Name)
	}
	if len(names) == 0 {
		names = []string{"(no branches)"}
	}
	return names
}

func remoteNames(remotes []git.Remote) []string {
	var names []string
	for _, r := range remotes {
		names = append(names, fmt.Sprintf("%s (%s)", r.Name, r.FetchURL))
	}
	if len(names) == 0 {
		names = []string{"(no remotes)"}
	}
	return names
}

// NewMainModel creates a new main model
func NewMainModel(commits []git.Commit) MainModel {
	content := "Welcome to GitForge TUI!\n\n"
	if len(commits) > 0 {
		content += "Recent commits:\n"
		for i, c := range commits {
			if i >= 10 {
				break
			}
			hash := c.Hash
			if len(hash) > 7 {
				hash = hash[:7]
			}
			content += fmt.Sprintf("  %s %s\n", hash, c.Subject)
		}
	} else {
		content += "No commits yet. Make your first commit!\n"
	}
	content += "\nPress Tab to switch panels, q to quit."

	return MainModel{
		content:  content,
		focused:  false,
		viewMode: "log",
	}
}

// NewDetailModel creates a new detail model
func NewDetailModel() DetailModel {
	return DetailModel{
		content: "Select a commit or file to see details here.\n\nPress Tab to navigate between panels.",
		focused: false,
	}
}

type dataRefreshedMsg struct {
	status   git.Status
	branches []git.Branch
	remotes  []git.Remote
	commits  []git.Commit
}

type actionResultMsg string

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case dataRefreshedMsg:
		m.status = msg.status
		m.branches = msg.branches
		m.remotes = msg.remotes
		m.commits = msg.commits
		if m.repo != nil {
			m.sidebar = NewSidebarModel(m.repo.Workdir(), msg.branches, msg.remotes)
		}
		m.main = NewMainModel(msg.commits)
		return m, nil

	case actionResultMsg:
		m.detail.content = string(msg)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "tab":
			m.focus = (m.focus + 1) % 3
			m.updateFocus()
			return m, nil

		case "shift+tab":
			m.focus = (m.focus - 1 + 3) % 3
			m.updateFocus()
			return m, nil

		case "up", "k":
			if m.focus == 0 {
				m.sidebar.MoveUp()
			}
			return m, nil

		case "down", "j":
			if m.focus == 0 {
				m.sidebar.MoveDown()
			}
			return m, nil

		case "enter":
			if m.focus == 0 {
				m.handleSidebarSelect()
			}
			return m, nil

		case "r":
			return m, m.refreshData()

		case "s":
			return m, m.stageAll()

		case "c":
			return m, m.commit()

		case "p":
			return m, m.push()

		case "P":
			return m, m.pull()
		}

	case tea.MouseMsg:
		return m, nil
	}

	return m, nil
}

func (m *Model) updateFocus() {
	m.sidebar.focused = m.focus == 0
	m.main.focused = m.focus == 1
	m.detail.focused = m.focus == 2
}

func (m *Model) handleSidebarSelect() {
	item := m.sidebar.SelectedItem()
	if item == "" {
		return
	}

	switch item {
	case "Stage All":
		m.detail.content = "Press 's' to stage all modified files."
	case "Commit":
		m.detail.content = "Run 'gitforge commit -m <msg>' in terminal to commit."
	case "Push":
		m.detail.content = "Press 'p' to push to remote."
	case "Pull":
		m.detail.content = "Press 'P' to pull from remote."
	case "Fetch":
		m.detail.content = "Press 'r' to refresh / fetch."
	case "Stash":
		m.detail.content = "Run 'gitforge stash' in terminal to stash changes."
	default:
		m.detail.content = fmt.Sprintf("Selected: %s", item)
	}
}

func (m *Model) refreshData() tea.Cmd {
	return func() tea.Msg {
		if m.repo == nil {
			return nil
		}
		status, _ := m.repo.Status()
		branches, _ := m.repo.Branches()
		remotes, _ := m.repo.Remotes()
		commits, _ := m.repo.Log(&git.LogOptions{Limit: 20})
		return dataRefreshedMsg{
			status:   status,
			branches: branches,
			remotes:  remotes,
			commits:  commits,
		}
	}
}

func (m *Model) stageAll() tea.Cmd {
	return func() tea.Msg {
		if m.repo == nil {
			return actionResultMsg("No repository open")
		}
		_ = m.repo.Add(".")
		status, _ := m.repo.Status()
		branches, _ := m.repo.Branches()
		remotes, _ := m.repo.Remotes()
		commits, _ := m.repo.Log(&git.LogOptions{Limit: 20})
		return dataRefreshedMsg{
			status:   status,
			branches: branches,
			remotes:  remotes,
			commits:  commits,
		}
	}
}

func (m *Model) commit() tea.Cmd {
	return func() tea.Msg {
		return actionResultMsg("Run 'gitforge commit -m <msg>' to commit.")
	}
}

func (m *Model) push() tea.Cmd {
	return func() tea.Msg {
		if m.repo != nil && len(m.remotes) > 0 {
			err := m.repo.Push(m.remotes[0].Name, []string{})
			if err != nil {
				return actionResultMsg(fmt.Sprintf("Push failed: %v", err))
			}
			return actionResultMsg("Push successful!")
		}
		return actionResultMsg("No remote configured to push to")
	}
}

func (m *Model) pull() tea.Cmd {
	return func() tea.Msg {
		if m.repo != nil && len(m.remotes) > 0 {
			err := m.repo.Pull(m.remotes[0].Name, "")
			if err != nil {
				return actionResultMsg(fmt.Sprintf("Pull failed: %v", err))
			}
			return actionResultMsg("Pull successful!")
		}
		return actionResultMsg("No remote configured to pull from")
	}
}

// View implements tea.Model
func (m Model) View() string {
	if m.quitting {
		return "Thanks for using GitForge! 👋\n"
	}

	if m.width == 0 {
		return "Loading..."
	}

	sidebarWidth := 30
	detailWidth := 40
	mainWidth := m.width - sidebarWidth - detailWidth - 6

	if mainWidth < 40 {
		mainWidth = 40
		detailWidth = m.width - sidebarWidth - mainWidth - 6
		if detailWidth < 20 {
			detailWidth = 20
		}
	}

	sidebarStyle := sidebarStyle.Width(sidebarWidth)
	mainStyle := mainStyle.Width(mainWidth)
	detailStyle := detailStyle.Width(detailWidth)

	if m.sidebar.focused {
		sidebarStyle = focusedBorder.Copy().Width(sidebarWidth)
	}
	if m.main.focused {
		mainStyle = focusedBorder.Copy().Width(mainWidth)
	}
	if m.detail.focused {
		detailStyle = focusedBorder.Copy().Width(detailWidth)
	}

	sidebar := sidebarStyle.Render(m.sidebar.View())
	main := mainStyle.Render(m.main.View())
	detail := detailStyle.Render(m.detail.View())

	help := helpStyle.Render("Tab/Shift+Tab: Switch panels  •  ↑/↓: Navigate  •  Enter: Select  •  r: Refresh  •  s: Stage all  •  c: Commit  •  p: Push  •  P: Pull  •  q: Quit")

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main, detail) + "\n" + help
}

// SidebarModel methods
func (s *SidebarModel) MoveUp() {
	if s.selected > 0 {
		s.selected--
	}
}

func (s *SidebarModel) MoveDown() {
	if s.selected < len(s.items)-1 {
		s.selected++
	}
}

func (s SidebarModel) SelectedItem() string {
	if s.selected >= 0 && s.selected < len(s.items) {
		return s.items[s.selected]
	}
	return ""
}

func (s SidebarModel) View() string {
	var items []string
	for i, item := range s.items {
		style := itemStyle
		if i == s.selected && s.focused {
			style = selectedItemStyle
		}
		items = append(items, style.Render(item))
	}
	return lipgloss.JoinVertical(lipgloss.Left, items...)
}

// MainModel methods
func (m MainModel) View() string {
	return m.content
}

// DetailModel methods
func (d DetailModel) View() string {
	return d.content
}

// Subcommands

func NewWebCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the GitForge web UI server",
		Long:  `Start the GitForge web UI server for browser-based repository management.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🌐 GitForge Web UI is currently in development.")
			fmt.Println("Use 'gitforge tui' to launch the interactive terminal interface.")
			return nil
		},
	}
	cmd.Flags().IntP("port", "p", 8080, "Port to run the web server on")
	cmd.Flags().StringP("host", "H", "127.0.0.1", "Host to bind the web server to")
	return cmd
}

func NewInitCmd() *cobra.Command {
	var bare bool

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new Git repository",
		Long:  `Initialize a new Git repository with GitForge's recommended defaults.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}
			repo, err := git.InitRepo(path, bare)
			if err != nil {
				return err
			}
			fmt.Printf("📁 Initialized Git repository at %s\n", repo.Workdir())
			return nil
		},
	}
	cmd.Flags().BoolVar(&bare, "bare", false, "Create a bare repository")
	return cmd
}

func NewCloneCmd() *cobra.Command {
	var branch string
	var depth int

	cmd := &cobra.Command{
		Use:   "clone <url> [path]",
		Short: "Clone a repository",
		Long:  `Clone a repository from a remote URL.`,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			path := ""
			if len(args) > 1 {
				path = args[1]
			} else {
				parts := strings.Split(strings.TrimSuffix(url, ".git"), "/")
				path = parts[len(parts)-1]
			}
			fmt.Printf("📥 Cloning %s into %s...\n", url, path)
			opts := &git.CloneOptions{
				Branch: branch,
				Depth:  depth,
			}
			repo, err := git.CloneRepo(context.Background(), url, path, opts)
			if err != nil {
				return err
			}
			fmt.Printf("✅ Repository cloned successfully to %s\n", repo.Workdir())
			return nil
		},
	}
	cmd.Flags().StringVar(&branch, "branch", "", "Branch to checkout")
	cmd.Flags().IntVar(&depth, "depth", 0, "Shallow clone depth")
	return cmd
}

func NewStatusCmd() *cobra.Command {
	var short bool
	var porcelain bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show repository status",
		Long:  `Show the working tree status with staged, unstaged, and untracked files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := git.OpenRepo(".")
			if err != nil {
				return err
			}

			if porcelain {
				out, err := repo.StatusRaw(true)
				if err != nil {
					return err
				}
				fmt.Print(out)
				return nil
			}

			status, err := repo.Status()
			if err != nil {
				return err
			}

			branch, _ := repo.Branch()
			if short {
				for _, f := range status.Staged {
					fmt.Printf("M  %s\n", f)
				}
				for _, f := range status.Unstaged {
					fmt.Printf(" M %s\n", f)
				}
				for _, f := range status.Untracked {
					fmt.Printf("?? %s\n", f)
				}
				for _, f := range status.Conflicted {
					fmt.Printf("UU %s\n", f)
				}
				return nil
			}

			fmt.Println("📊 Repository Status")
			fmt.Println("━━━━━━━━━━━━━━━━━━━")
			if branch != "" {
				fmt.Printf("On branch %s\n\n", branch)
			}

			if status.IsClean() {
				fmt.Println("nothing to commit, working tree clean")
				return nil
			}

			if len(status.Staged) > 0 {
				fmt.Println("Changes to be committed:")
				fmt.Println("  (use \"git restore --staged <file>...\" to unstage)")
				for _, f := range status.Staged {
					fmt.Printf("    \033[32mmodified:   %s\033[0m\n", f)
				}
				fmt.Println()
			}

			if len(status.Unstaged) > 0 {
				fmt.Println("Changes not staged for commit:")
				fmt.Println("  (use \"git add <file>...\" to update what will be committed)")
				for _, f := range status.Unstaged {
					fmt.Printf("    \033[31mmodified:   %s\033[0m\n", f)
				}
				fmt.Println()
			}

			if len(status.Untracked) > 0 {
				fmt.Println("Untracked files:")
				fmt.Println("  (use \"git add <file>...\" to include in what will be committed)")
				for _, f := range status.Untracked {
					fmt.Printf("    \033[31m%s\033[0m\n", f)
				}
				fmt.Println()
			}

			if len(status.Conflicted) > 0 {
				fmt.Println("Unmerged paths:")
				for _, f := range status.Conflicted {
					fmt.Printf("    \033[35mboth modified:   %s\033[0m\n", f)
				}
				fmt.Println()
			}

			return nil
		},
	}
	cmd.Flags().BoolVarP(&short, "short", "s", false, "Short format")
	cmd.Flags().BoolVar(&porcelain, "porcelain", false, "Machine-readable output")
	return cmd
}

func NewLogCmd() *cobra.Command {
	var maxCount int
	var oneline bool
	var author string
	var grep string

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show commit history",
		Long:  `Show the commit history with clean formatting.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := git.OpenRepo(".")
			if err != nil {
				return err
			}

			opts := &git.LogOptions{
				Limit:  maxCount,
				Author: author,
				Grep:   grep,
			}
			commits, err := repo.Log(opts)
			if err != nil {
				return err
			}

			if len(commits) == 0 {
				fmt.Println("No commits yet.")
				return nil
			}

			if oneline {
				for _, c := range commits {
					hash := c.Hash
					if len(hash) > 7 {
						hash = hash[:7]
					}
					fmt.Printf("\033[33m%s\033[0m %s\n", hash, c.Subject)
				}
				return nil
			}

			fmt.Println("📜 Commit History")
			fmt.Println("━━━━━━━━━━━━━━━")
			for _, c := range commits {
				hash := c.Hash
				if len(hash) > 7 {
					hash = hash[:7]
				}
				fmt.Printf("* \033[33m%s\033[0m %s\n", hash, c.Subject)
				fmt.Printf("| Author: %s <%s>\n", c.Author.Name, c.Author.Email)
				fmt.Printf("| Date:   %s\n", c.Date.Format("Mon Jan 02 15:04:05 2006 -0700"))
				if c.Body != "" {
					fmt.Printf("|\n|   %s\n", strings.ReplaceAll(c.Body, "\n", "\n|   "))
				}
				fmt.Println("|")
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&maxCount, "max-count", "n", 10, "Limit number of commits")
	cmd.Flags().BoolVar(&oneline, "oneline", false, "One line per commit")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author")
	cmd.Flags().StringVar(&grep, "grep", "", "Filter by commit message")
	return cmd
}

func NewBranchCmd() *cobra.Command {
	var showAll bool
	var deleteBranch bool
	var forceDelete bool

	cmd := &cobra.Command{
		Use:   "branch [name]",
		Short: "Manage branches",
		Long:  `List, create, or delete branches.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := git.OpenRepo(".")
			if err != nil {
				return err
			}

			if deleteBranch {
				if len(args) == 0 {
					return fmt.Errorf("branch name required to delete")
				}
				if err := repo.DeleteBranch(args[0], forceDelete); err != nil {
					return err
				}
				fmt.Printf("Deleted branch %s\n", args[0])
				return nil
			}

			if len(args) == 1 {
				if err := repo.CreateBranch(args[0], ""); err != nil {
					return err
				}
				fmt.Printf("Created branch %s\n", args[0])
				return nil
			}

			branches, err := repo.Branches()
			if err != nil {
				return err
			}

			fmt.Println("🌿 Branches")
			fmt.Println("━━━━━━━━━━")
			for _, b := range branches {
				prefix := "  "
				if b.IsCurrent {
					prefix = "* \033[32m"
					fmt.Printf("%s%s\033[0m", prefix, b.Name)
				} else {
					fmt.Printf("%s%s", prefix, b.Name)
				}
				if b.Upstream != "" {
					fmt.Printf(" -> %s", b.Upstream)
				}
				fmt.Println()
			}

			if showAll {
				remotes, err := repo.RemoteBranches()
				if err == nil && len(remotes) > 0 {
					fmt.Println("\nRemote branches:")
					for _, rb := range remotes {
						fmt.Printf("  \033[31m%s\033[0m\n", rb.Name)
					}
				}
			}

			return nil
		},
	}
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all branches (including remote)")
	cmd.Flags().BoolVarP(&deleteBranch, "delete", "d", false, "Delete branch")
	cmd.Flags().BoolVarP(&forceDelete, "force", "D", false, "Force delete branch")
	return cmd
}

func NewCommitCmd() *cobra.Command {
	var message string
	var stageAll bool

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Create a commit",
		Long:  `Create a new commit with staged changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := git.OpenRepo(".")
			if err != nil {
				return err
			}

			if message == "" {
				return fmt.Errorf("commit message required (-m)")
			}

			if stageAll {
				if err := repo.Add("."); err != nil {
					return fmt.Errorf("staging failed: %w", err)
				}
			}

			hash, err := repo.Commit(message, nil)
			if err != nil {
				return err
			}

			if len(hash) > 7 {
				hash = hash[:7]
			}
			fmt.Printf("📝 Commit created: %s %s\n", hash, message)
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message")
	cmd.Flags().BoolVarP(&stageAll, "all", "a", false, "Stage all modified files")
	return cmd
}

func NewDiffCmd() *cobra.Command {
	var cached bool
	var file string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show changes",
		Long:  `Show differences between commits, working tree, and index.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := git.OpenRepo(".")
			if err != nil {
				return err
			}

			diff, err := repo.Diff(&git.DiffOptions{
				Cached: cached,
				File:   file,
			})
			if err != nil {
				return err
			}

			if strings.TrimSpace(diff.Raw) == "" {
				return nil
			}

			fmt.Println("📝 Diff")
			fmt.Println("━━━━━━━━")
			fmt.Println(diff.Raw)
			return nil
		},
	}
	cmd.Flags().BoolVar(&cached, "cached", false, "Show staged changes")
	cmd.Flags().StringVar(&file, "file", "", "Show diff for specific file")
	return cmd
}

func NewRemoteCmd() *cobra.Command {
	var addRemote string
	var removeRemote string

	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Manage remotes",
		Long:  `Manage remote repositories.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := git.OpenRepo(".")
			if err != nil {
				return err
			}

			if removeRemote != "" {
				if err := repo.RemoveRemote(removeRemote); err != nil {
					return err
				}
				fmt.Printf("Removed remote %s\n", removeRemote)
				return nil
			}

			if addRemote != "" {
				parts := strings.Fields(addRemote)
				if len(parts) < 2 {
					return fmt.Errorf("add requires '<name> <url>'")
				}
				if err := repo.AddRemote(parts[0], parts[1]); err != nil {
					return err
				}
				fmt.Printf("Added remote %s (%s)\n", parts[0], parts[1])
				return nil
			}

			remotes, err := repo.Remotes()
			if err != nil {
				return err
			}

			if len(remotes) == 0 {
				fmt.Println("No remotes configured.")
				return nil
			}

			fmt.Println("🔗 Remotes")
			fmt.Println("━━━━━━━━")
			for _, r := range remotes {
				if r.FetchURL != "" {
					fmt.Printf("%-10s %s (fetch)\n", r.Name, r.FetchURL)
				}
				if r.PushURL != "" {
					fmt.Printf("%-10s %s (push)\n", r.Name, r.PushURL)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&addRemote, "add", "", "Add remote (format: 'name url')")
	cmd.Flags().StringVar(&removeRemote, "remove", "", "Remove remote by name")
	return cmd
}

func NewStashCmd() *cobra.Command {
	var message string
	var listStashes bool
	var popStash bool
	var dropStash bool

	cmd := &cobra.Command{
		Use:   "stash",
		Short: "Stash changes",
		Long:  `Stash changes in a dirty working directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := git.OpenRepo(".")
			if err != nil {
				return err
			}

			if popStash {
				if err := repo.StashPop(); err != nil {
					return err
				}
				fmt.Println("✅ Stash popped successfully!")
				return nil
			}

			if dropStash {
				if err := repo.StashDrop(0); err != nil {
					return err
				}
				fmt.Println("✅ Stash dropped successfully!")
				return nil
			}

			if listStashes {
				stashes, err := repo.StashList()
				if err != nil {
					return err
				}
				if len(stashes) == 0 {
					fmt.Println("No stashes found.")
					return nil
				}
				for _, s := range stashes {
					fmt.Printf("%s: %s\n", s.Index, s.Message)
				}
				return nil
			}

			if err := repo.Stash(message); err != nil {
				return err
			}
			fmt.Println("📦 Stashed changes successfully!")
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Stash message")
	cmd.Flags().BoolVar(&listStashes, "list", false, "List stashes")
	cmd.Flags().BoolVar(&popStash, "pop", false, "Pop latest stash")
	cmd.Flags().BoolVar(&dropStash, "drop", false, "Drop latest stash")
	return cmd
}

func NewTagCmd() *cobra.Command {
	var message string
	var deleteTag string

	cmd := &cobra.Command{
		Use:   "tag [name]",
		Short: "Manage tags",
		Long:  `Create, list, or delete tags.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := git.OpenRepo(".")
			if err != nil {
				return err
			}

			if deleteTag != "" {
				if err := repo.DeleteTag(deleteTag); err != nil {
					return err
				}
				fmt.Printf("Deleted tag %s\n", deleteTag)
				return nil
			}

			if len(args) == 1 {
				if err := repo.CreateTag(args[0], "HEAD", message); err != nil {
					return err
				}
				fmt.Printf("Created tag %s\n", args[0])
				return nil
			}

			tags, err := repo.Tags()
			if err != nil {
				return err
			}
			if len(tags) == 0 {
				fmt.Println("No tags found.")
				return nil
			}
			fmt.Println("🏷️  Tags")
			fmt.Println("━━━━━━━")
			for _, t := range tags {
				fmt.Println(t.Name)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Tag message")
	cmd.Flags().StringVarP(&deleteTag, "delete", "d", "", "Delete tag by name")
	return cmd
}

func NewConfigCmd() *cobra.Command {
	var getKey string
	var setKey string
	var listAll bool

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure Git",
		Long:  `Get or set Git repository configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := git.OpenRepo(".")
			if err != nil {
				return err
			}

			if getKey != "" {
				val, err := repo.GetConfig(getKey)
				if err != nil {
					return err
				}
				fmt.Println(val)
				return nil
			}

			if setKey != "" {
				parts := strings.SplitN(setKey, "=", 2)
				if len(parts) < 2 {
					return fmt.Errorf("set requires 'key=value'")
				}
				if err := repo.SetConfig(parts[0], parts[1]); err != nil {
					return err
				}
				fmt.Printf("Set %s=%s\n", parts[0], parts[1])
				return nil
			}

			cfg, err := repo.Config()
			if err != nil {
				return err
			}
			fmt.Println("⚙️  Git Configuration")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━")
			if cfg.UserName != "" {
				fmt.Printf("user.name=%s\n", cfg.UserName)
			}
			if cfg.UserEmail != "" {
				fmt.Printf("user.email=%s\n", cfg.UserEmail)
			}
			if cfg.DefaultBranch != "" {
				fmt.Printf("init.defaultBranch=%s\n", cfg.DefaultBranch)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&getKey, "get", "", "Get config value")
	cmd.Flags().StringVar(&setKey, "set", "", "Set config value (key=val)")
	cmd.Flags().BoolVar(&listAll, "list", false, "List config")
	return cmd
}

func NewPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
		Long:  `Manage GitForge plugins.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🔌 GitForge Plugin system is planned for a future release.")
			return nil
		},
	}
	cmd.Flags().String("install", "", "Install plugin")
	cmd.Flags().String("remove", "", "Remove plugin")
	cmd.Flags().Bool("list", false, "List plugins")
	return cmd
}

func NewDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run system diagnostics",
		Long:  `Run system health checks and diagnostics.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🩺 GitForge Doctor")
			fmt.Println("━━━━━━━━━━━━━━━")

			// Check git binary
			gitPath, err := exec.LookPath("git")
			if err != nil {
				fmt.Println("❌ Git: Not found in PATH")
			} else {
				out, _ := exec.Command(gitPath, "--version").Output()
				fmt.Printf("✅ Git: %s\n", strings.TrimSpace(string(out)))
			}

			// Check GitForge version and build info
			fmt.Printf("✅ GitForge: %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)

			// Check if inside git repo
			repo, err := git.OpenRepo(".")
			if err != nil {
				fmt.Println("⚠️  Repository: Current directory is not inside a Git repository")
			} else {
				fmt.Printf("✅ Repository: %s\n", repo.Workdir())
				remotes, _ := repo.Remotes()
				if len(remotes) > 0 {
					fmt.Printf("✅ Remotes: %d configured (%s)\n", len(remotes), remotes[0].Name)
				} else {
					fmt.Println("ℹ️  Remotes: None configured")
				}
			}

			// Check SSH
			if _, err := exec.LookPath("ssh"); err == nil {
				fmt.Println("✅ SSH: Client available")
			} else {
				fmt.Println("ℹ️  SSH: No ssh client found")
			}

			// Check GPG
			if _, err := exec.LookPath("gpg"); err == nil {
				fmt.Println("✅ GPG: Client available")
			} else {
				fmt.Println("ℹ️  GPG: No gpg client found")
			}

			fmt.Println("\nAll system checks completed!")
			return nil
		},
	}
	cmd.Flags().Bool("fix", false, "Auto-fix issues")
	return cmd
}

func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  `Display GitForge version information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("GitForge version %s\n", Version)
			fmt.Printf("Go runtime: %s\n", runtime.Version())
			fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
	return cmd
}
