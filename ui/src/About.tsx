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
            <p>
                The source code for this site is available on <a href="https://github.com/freeeve/deep-libby">github</a>.
            </p>
            <h2>Changelog</h2>
            <h3>Version 2025-02-28:</h3>
            <ul>
                <li>
                    Added Hardcover integration. ISBN search now working as well.
                </li>
            </ul>
            <h3>Version 2025-02-14:</h3>
            <ul>
                <li>
                    Added link icon to open availability page in a new tab.
                </li>
            </ul>
            <h3>Version 2025-02-13:</h3>
            <ul>
                <li>
                    Added publisher name to search results as well as inline search.
                </li>
            </ul>
            <h3>Version 2025-01-06:</h3>
            <ul>
                <li>
                    Temporarily removed diff/unique/indersect pages, as they were not working correctly and caused the
                    server to be unstable. I will re-enable them when I have time to optimize them. Sorry!
                </li>
            </ul>
            <h3>Version 2024-12-10:</h3>
            <ul>
                <li>
                    Fixed an availability counts/formats bug introduced in last build.
                    Converted media info to be stored on badger as well.
                </li>
            </ul>
            <h3>Version 2024-11-14:</h3>
            <ul>
                <li>
                    Using Badger for the backend datastore, to make it cheaper to run (memory usage much reduced).
                </li>
            </ul>
            <h3>Version 2024-09-06:</h3>
            <ul>
                <li>
                    Availability can now be updated live on the availability page (two links you can click on the right
                    hand side). It will load with stale data, then update with the latest data from the Overdrive
                    API--there is a delay to prevent overwhelming the Overdrive APIs.
                    Unfortunately, if a title is recently added to a collection, we won't know until the collection
                    refresh (which takes longer), but at least the owned/available/holds counts will be updated for the
                    collections already known.
                </li>
                <li>Memory optimization progress: Strings (titles, descriptions, etc.) are now stored in a memory mapped
                    file, so the OS can manage how much memory it wants to page in. It is already down to ~10gb, didn't
                    expect that to be such a quick win.
                </li>
            </ul>
            <h3>Version 2024-09-05:</h3>
            <ul>
                <li>UI: fixed a bug where the availability page's "open in libby" links didn't work. Main reason for
                    release.
                </li>
                <li>Search: pushed sorting results to the client side, which saves 100-150ms of cpu for the worst cases,
                    and
                    will eventually make this easier to scale, and it is transparent to the user.
                </li>
                <li>I'm starting to optimize memory usage on the service. Currently the data is all loaded into memory
                    and served directly from there. It takes ~17gb of memory, after some tightening of types (don't
                    really care if a book has &gt;16k available at a given library). I'd like to get memory down
                    to &lt;14gb, as a first goal. Then &lt;6gb. Notably these sizes are a couple of gigs less than ec2
                    memory sizes, so each time it gets down below the next level, it'll cost a bit less to run. I doubt
                    I'll be able to get it down to ~3gb, but that would be ideal. The ngram roaring bitmaps (used for
                    the search) alone currently take only about ~700mb. I love this part of software engineering.
                </li>
            </ul>
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
