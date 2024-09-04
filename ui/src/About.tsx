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
            <h3>Version 2024-09-04:</h3>
            <ul>
                <li>Changelog: I'll start tracking code updates here.</li>
                <li>Search results (sorting): Worked on search results ordering again. Now it is based on longest common
                    substring (desc) and then library count (desc).
                </li>
                <li>Support for advantage accounts within consortiums, and favorites for them: you probably will need to
                    update your favorites to exclude the cards in the consortium you don't have. Note that data is still
                    being indexed for some of these libraries.
                </li>
                <li>Unicode normalization for better foreign language search (umlauts, accents, etc., can be searched
                    with or without).
                </li>
                <li>Added better support for formats--now tracked at the per-library level, since I discovered that the
                    same media id doesn't mean it has the same formats everywhere. It appears to be somewhat regional,
                    kindle being excluded outside the US, for example.
                </li>
                <li>UI: Finally added menu on more pages. Sorry, my react UI skills are not up to snuff.</li>
                <li>UI: Changed the way you open things in libby (now you click the title or library name on several
                    screens), to save horizontal screen real estate.
                </li>
            </ul>
        </div>
    )
}
