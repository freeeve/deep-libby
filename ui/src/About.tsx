import SearchMedia from "./SearchMedia.tsx";

export default function About() {
    return (
        <div>
            <SearchMedia></SearchMedia>
            <h1>About</h1>
            <p>This site is unaffiliated with Libby / Overdrive.</p>
            <p>
                Please send complaints to <strong>/dev/null</strong> and praise/constructive feedback to eve-f on
                reddit.
            </p>
            <h2>Changelog</h2>
            <h3>Version 2024-09-03:</h3>
            <ul>
                <li>changelog: I'll start tracking code updates here.</li>
                <li>search results (sorting): worked on search results ordering again. now it is based on longest common
                    substring (desc) and then library count (desc).
                </li>
                <li>support for advantage accounts within consortiums, and favorites for them: you probably will need to
                    update your favorites to exclude the cards in the consortium you don't have. note that data is still
                    being indexed for some of these libraries.
                </li>
                <li>added better support for formats--now tracked at the per-library level, since I discovered that the
                    same media id doesn't mean it has the same formats everywhere. it appears to be somewhat regional,
                    kindle being excluded outside the US, for example.
                </li>
                <li>UI: finally added menu on more pages. sorry, my react UI skills are not up to snuff.</li>

                <li>UI: changed the way you open things in libby (now you click the title or library name), to save
                    horizontal screen real estate.
                </li>
            </ul>
        </div>
    )
}
