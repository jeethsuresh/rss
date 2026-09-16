import SwiftUI

struct StoryDetailView: View {
    @Environment(SessionStore.self) private var session
    @State private var story: Story
    @State private var busy = false

    init(story: Story) { _story = State(initialValue: story) }

    var body: some View {
        List {
            Section("Summary · \(story.memberCount) sources") {
                Text(story.title)
                    .font(.title2.bold())
                Text(story.summary)
                    .font(.system(.body, design: .serif))
                    .padding(.vertical, 6)
                HStack {
                    Button("Useful", systemImage: story.vote == .up ? "hand.thumbsup.fill" : "hand.thumbsup") {
                        Task { await vote(story.vote == .up ? .none : .up) }
                    }
                    Spacer()
                    Button("Not useful", systemImage: story.vote == .down ? "hand.thumbsdown.fill" : "hand.thumbsdown") {
                        Task { await vote(story.vote == .down ? .none : .down) }
                    }
                }
                .buttonStyle(.borderless)
            }
            Section("Sources") {
                ForEach(story.articles ?? []) { article in
                    NavigationLink { ReaderView(article: article) } label: { ArticleRow(article: article) }
                        .swipeActions(edge: .leading) {
                            Button("Relevant", systemImage: story.articleVotes?[article.id] == .up ? "hand.thumbsup.fill" : "hand.thumbsup") {
                                Task { await vote(article: article, vote: .up) }
                            }
                            .tint(.green)
                        }
                        .swipeActions(edge: .trailing) {
                            Button("Not relevant", systemImage: story.articleVotes?[article.id] == .down ? "hand.thumbsdown.fill" : "hand.thumbsdown") {
                                Task { await vote(article: article, vote: .down) }
                            }
                            .tint(.orange)
                        }
                }
            }
        }
        .scrollContentBackground(.hidden)
        .background { JoyfulBackdrop() }
        .navigationTitle(story.title)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            Menu {
                Button(story.isRead ? "Mark unread" : "Mark read", systemImage: story.isRead ? "circle" : "checkmark") {
                    Task { await mutate { story = try await session.api.setStoryRead(id: story.id, read: !story.isRead) } }
                }
                Button(story.isStarred ? "Unstar" : "Star", systemImage: "star") {
                    Task { await mutate { story = try await session.api.toggleStoryStar(id: story.id) } }
                }
                Button("Split story", systemImage: "square.split.2x1") {
                    Task { await mutate { _ = try await session.api.splitStory(id: story.id) } }
                }
            } label: { Image(systemName: "ellipsis.circle") }
        }
        .task {
            await mutate { story = try await session.api.story(id: story.id) }
        }
        .disabled(busy)
    }

    private func vote(_ vote: StoryVote) async {
        await mutate { story = try await session.api.voteStory(id: story.id, vote: vote) }
    }

    private func vote(article: Article, vote: StoryVote) async {
        let next: StoryVote = story.articleVotes?[article.id] == vote ? .none : vote
        await mutate { story = try await session.api.voteArticle(storyID: story.id, articleID: article.id, vote: next) }
    }

    private func mutate(_ operation: () async throws -> Void) async {
        busy = true; defer { busy = false }
        do { try await operation() } catch { session.report(error) }
    }
}
