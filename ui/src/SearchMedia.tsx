import {Virtuoso} from "react-virtuoso";
import React, {useEffect, useRef, useState} from "react";
import {useNavigate} from "react-router-dom";

interface SearchMedia {
    id: string;
    title: string;
    creators: { name: string, role: string }[];
    languages: string[];
    formats: string[];
    coverUrl: string;
    seriesName: string;
    seriesReadOrder: number;
    libraryCount: number;
}

interface SearchMediaProps {
    clickMedia?: Function;
}

export default function SearchMedia({clickMedia}: SearchMediaProps) {
    const [width, setWidth] = useState<number>(window.innerWidth);
    let debounceTimeoutId: any | null = null;
    let searchComponent: null | JSX.Element;

    function handleWindowSizeChange() {
        setWidth(window.innerWidth);
    }

    useEffect(() => {
        window.addEventListener('resize', handleWindowSizeChange);
        return () => {
            window.removeEventListener('resize', handleWindowSizeChange);
        }
    }, []);
    const isMobile = width <= 900;

    let baseUrl = window.location.origin;
    if (baseUrl === 'http://localhost:5173') {
        baseUrl = 'http://localhost:8080';
    }
    const [data, setData] = useState({results: []});
    const navigate = useNavigate(); // Get the history object
    const abortControllerRef = useRef(new AbortController());

    const search = (term: string, signal: AbortSignal) => {
        let url = new URL('/api/search', baseUrl);
        let params: any = {q: term};
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));

        return fetch(url, {
            method: 'GET',
            signal: signal,
        })
            .then(response => response.json())
            .catch((error) => {
                console.error('Error:', error);
            });
    };

    const handleInputChange = (event: React.ChangeEvent<HTMLInputElement>) => {

        console.log("handleInputChange " + event.target.value);
        // Abort any pending requests
        if (abortControllerRef.current) {
            console.log("aborting from handleInputChange");
            abortControllerRef.current.abort();
        }
        let term = event.target.value;
        if (term.length < 1) {
            setData({results: []});
            return
        }

        if (debounceTimeoutId) {
            clearTimeout(debounceTimeoutId);
        }

        const newAbortController = new AbortController();
        abortControllerRef.current = newAbortController;
        debounceTimeoutId = setTimeout(() => {
            search(term, newAbortController.signal)
                .then((data) => {
                    if (data) {
                        setData(data);
                        data.results.sort((a: SearchMedia, b: SearchMedia) => {
                            let lowerCaseTerm = term.toLowerCase();
                            console.log("lowerCaseTerm: " + lowerCaseTerm)
                            let lcsATitle = longestCommonSubstring(lowerCaseTerm, " " + a.title.toLowerCase() + " ");
                            console.log("lcsATitle: " + lcsATitle, " a.title: " + a.title.toLowerCase() + " term: " + lowerCaseTerm);
                            let lcsBTitle = longestCommonSubstring(lowerCaseTerm, " " + b.title.toLowerCase() + " ");
                            console.log("lcsBTitle: " + lcsBTitle, " b.title: " + b.title.toLowerCase() + " term: " + lowerCaseTerm);

                            let lcsAAuthor = longestCommonSubstring(lowerCaseTerm,
                                " " + a.creators.map(creator => creator.name).join(" ") + " ");
                            let lcsBAuthor = longestCommonSubstring(lowerCaseTerm,
                                " " + b.creators.map(creator => creator.name).join(" ") + " ");

                            let lcsASeries = longestCommonSubstring(lowerCaseTerm, " #" + a.seriesReadOrder + " " + a.seriesName.toLowerCase() + " ");
                            let lcsBSeries = longestCommonSubstring(lowerCaseTerm, " #" + b.seriesReadOrder + " " + b.seriesName.toLowerCase() + " ");

                            let lcsA = Math.max(lcsATitle, lcsASeries, lcsAAuthor);
                            let lcsB = Math.max(lcsBTitle, lcsBSeries, lcsBAuthor);
                            console.log("lcsA: " + lcsA + " lcsB: " + lcsB + " a.libraryCount: " + a.libraryCount + " b.libraryCount: " + b.libraryCount)
                            if (lcsA === lcsB) {
                                return b.libraryCount - a.libraryCount;
                            }
                            return lcsB - lcsA;
                        });
                    }
                })
                .catch((error) => {
                    if (error.name !== 'AbortError') {
                        console.error(error);
                    }
                });
        }, isMobile ? 700 : 50);
    };

    if (clickMedia === undefined) {
        clickMedia = (selectedOption: any) => {
            console.log("default handler, ", selectedOption);
        };
    }

    const longestCommonSubstring = (a: string, b: string) => {
        let m = a.length;
        let n = b.length;
        let dp = new Array(m + 1).fill(0).map(() => new Array(n + 1).fill(0));
        let max = 0;
        for (let i = 1; i <= m; i++) {
            for (let j = 1; j <= n; j++) {
                if (a[i - 1] === b[j - 1]) {
                    dp[i][j] = dp[i - 1][j - 1] + 1;
                    max = Math.max(max, dp[i][j]);
                }
            }
        }
        return max
    }

    const Row = (index: number) => {
        const result: SearchMedia = data.results[index];
        return (
            <div
                onClick={() => {
                    navigate('/availability/' + result.id);
                    clickMedia({id: result.id}, []);
                    data.results = [];
                }}
                style={{
                    backgroundColor: index % 2 === 0 ? '#333' : '#444',
                    display: 'flex',
                    justifyContent: 'space-between',
                    padding: isMobile ? 4 : 0,
                    cursor: 'pointer',
                }}>
                <div style={{textAlign: 'left', width: isMobile ? '60%' : '70%'}}>
                    <div><strong>{result.title}{result.seriesName !== "" &&
                        <span> (#{result.seriesReadOrder} in {result.seriesName})</span>}</strong></div>
                    <span>{isMobile ? '' : 'Creators:'} {result.creators.map((author) => author.name + ' (' + author.role + ')').join(', ')}</span>
                    <div>{isMobile ? '' : 'Languages:'} {result.languages ? result.languages.join(', ') : ''}</div>
                    <div>{isMobile ? '' : 'Formats:'} {result.formats ? result.formats.join(', ') : ''}</div>
                </div>
                <div style={{textAlign: 'right'}}>
                    <span
                        style={{
                            float: isMobile ? 'none' : "none",
                            verticalAlign: isMobile ? 'bottom' : 'top',
                            marginRight: isMobile ? 0 : 5
                        }}>Owned by {result.libraryCount} libraries.</span>
                    <img src={result.coverUrl}
                         alt={result.title}
                         width={0} height={0}
                         sizes="100vw"
                         style={{width: 'auto', height: '100px', float: 'right'}} // optional
                    />
                </div>
            </div>
        );
    };

    if (!isMobile) {
        searchComponent = <div style={{width: '100%'}}>
            <div style={{fontSize: 24, textDecoration: 'underline'}}>
                DeepLibby Search
                {/*
                <span style={{marginLeft: 50, cursor: 'pointer'}}>
                        <span onClick={() => navigate('/diff/')}>Library Diff</span>
                    </span>
                <span style={{marginLeft: 50, cursor: 'pointer'}}>
                        <span onClick={() => navigate('/intersect/')}>Library Intersect</span>
                    </span>
                <span style={{marginLeft: 50, cursor: 'pointer'}}>
                        <span onClick={() => navigate('/unique/')}>Library Unique</span>
                    </span>
                */}
                <span style={{marginLeft: 50, cursor: 'pointer'}}>
                        <span onClick={() => navigate('/libraries')}>Favorite Libraries</span>
                    </span>
                <span style={{marginLeft: 100, cursor: 'pointer'}}>
                        <span onClick={() => navigate('/about')}>About</span>
                    </span>
            </div>
            <input type="text"
                   placeholder="search here. inline filters for language, format, title, author. 'tomorrow zevin kindle english' for example"
                   style={{width: '100%', height: 50, fontSize: 24}}
                   onChange={handleInputChange}
            />
            {data.results && data.results.length > 0 && (
                <Virtuoso
                    style={{height: 650, width: '100%'}}
                    totalCount={data.results ? data.results.length : 0}
                    itemContent={Row}
                />
            )}
        </div>
    } else {
        searchComponent = <div style={{width: '100%'}}>
            <div style={{fontSize: 24, textDecoration: 'underline'}}>
                {/*
                <div style={{cursor: 'pointer'}}>
                    <div onClick={() => navigate('/diff/')}>Library Diff</div>
                </div>
                <div style={{cursor: 'pointer'}}>
                    <div onClick={() => navigate('/intersect/')}>Library Intersect</div>
                </div>
                <div style={{cursor: 'pointer'}}>
                    <div onClick={() => navigate('/unique/')}>Library Unique</div>
                </div>
                */}
                <div style={{cursor: 'pointer'}}>
                    <div onClick={() => navigate('/libraries')}>Favorite Libraries</div>
                </div>
                <div style={{cursor: 'pointer'}}>
                    <div onClick={() => navigate('/about')}>About</div>
                </div>
            </div>
            <div style={{fontSize: 24, paddingTop: 5}}>DeepLibby Search</div>
            <input type="text"
                   placeholder="search here. inline filters for language, format, title, author. 'tomorrow zevin kindle english' for example"
                   style={{width: '100%', height: 50, fontSize: 24}}
                   onChange={handleInputChange}
            />
            {data.results && data.results.length > 0 && (
                <Virtuoso
                    style={{height: 650, width: '100%'}}
                    totalCount={data.results ? data.results.length : 0}
                    itemContent={Row}
                />
            )}
        </div>;
    }


    return (
        <div>
            {searchComponent}
        </div>
    );
}