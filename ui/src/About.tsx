export default function About() {
    return (
        <div>
            <h1>About</h1>
            <p>This site is unaffiliated with Libby / Overdrive.</p>
            <p>
                Please send complaints to <strong>/dev/null</strong> and praise/constructive feedback to eve-f on
                reddit.
            </p>
            <p>Whipped up the UI over the last few days so expect significant changes and probable downtime as I continue to
                work on it.
                Currently, the results for "availability" and "holds" are stale by ~1 week, but still useful for my use
                case.</p>
            <h3>Roadmap:</h3>
            <ul>
                <li>refresh availability automatically (probably in 24-48h increments or something)</li>
                <li>better search, ability to filter by subject, language, format more easily</li>
                <li>ability to save favorite libraries (which will show at the top of results)</li>
            </ul>
        </div>
    )
}
