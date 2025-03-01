import {useState, useEffect} from "react";
import {AgGridReact} from "ag-grid-react";
import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-alpine.css';
import {ColDef} from "ag-grid-community";
import SearchMedia from "./SearchMedia.tsx";

interface SearchResult {
    id: string;
    title: string;
    subtitle: string;
    creators: { name: string, role: string }[];
    publisher: string;
    publisherId: number;
    languages: string[];
    formats: string[];
    description: string;
    coverUrl: string;
    seriesName: string;
    seriesReadOrder: number;
    libraryCount: number;
    availableNowFavorites: string | LibraryAvailability;
}

interface LibraryAvailability {
    libraryId: string;
    libraryName: string;
    libbyLink: string;
    totalLibraries: number;
    mostAvailableCopies?: number;
}

export default function Hardcover() {
    let baseUrl = window.location.origin;
    if (baseUrl === 'http://localhost:5173') {
        baseUrl = 'http://localhost:8080';
    }
    const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
    const [username, setUsername] = useState<string>("");
    const [additionalFilters, setAdditionalFilters] = useState<string>("");
    const [favorites, setFavorites] = useState<string[]>([]);
    const [loading, setLoading] = useState<boolean>(false);

    const columnDefs: ColDef[] = [
        {
            headerName: 'Title (click to view availability)',
            field: 'title',
            minWidth: 250,
            cellRenderer: (params: any) => {
                return (<a href={`/availability/${params.data.id}`}>{params.value}</a>);
            }
        },
        {
            headerName: 'Available Now (Favorites)',
            field: 'availableNowFavorites',
            minWidth: 250,
            sort: 'asc',
            cellRenderer: (params: any) => {
                if (typeof params.value === 'string') {
                    return params.value;
                } else {
                    return (
                        <span>
                            <a href={params.value.libbyLink} style={{cursor: 'pointer'}}>
                                {params.value.libraryName}
                            </a>
                            {params.value.totalLibraries > 1 &&
                                <span>
                                    <span>&nbsp;and {params.value.totalLibraries - 1}</span>
                                    <span>&nbsp;other{params.value.totalLibraries > 2 ? 's' : ''}</span>
                                </span>
                            }
                        </span>
                    );
                }
            },
            comparator: (valueA, valueB) => {
                if (typeof valueA === 'string' && typeof valueB === 'string') {
                    return valueA.localeCompare(valueB);
                } else if (typeof valueA === 'object' && typeof valueB === 'object') {
                    return valueB.totalLibraries - valueA.totalLibraries;
                } else if (typeof valueA === 'string') {
                    return 1;
                } else {
                    return -1;
                }
            }
        },
        {headerName: 'Total Libraries', field: 'libraryCount', minWidth: 150},
        {
            headerName: 'Creators',
            field: 'creators',
            minWidth: 200,
            valueFormatter: (params) => params.value.map((creator: any) => `${creator.name} (${creator.role})`).join(', ')
        },
        {headerName: 'Publisher', field: 'publisher', minWidth: 100},
        {
            headerName: 'Languages',
            field: 'languages',
            minWidth: 100,
            valueFormatter: (params) => params.value.join(', ')
        },
        {headerName: 'Formats', field: 'formats', minWidth: 150, valueFormatter: (params) => params.value.join(', ')},
    ];

    const handleSearch = () => {
        const sanitizedUsername = username.trim();
        const sanitizedFilters = additionalFilters.trim();

        setLoading(true);
        let url = new URL('/api/search-hardcover', baseUrl);
        url.searchParams.append('username', sanitizedUsername);
        url.searchParams.append('additionalFilters', sanitizedFilters);

        fetch(url.toString())
            .then(response => response.json())
            .then(async data => {
                if (data.length === 0) {
                    setLoading(false);
                    return;
                }
                const mediaIds = data.map((item: SearchResult) => item?.id).filter((id: any) => id !== undefined);
                const updatedResults = data.map((item: SearchResult) => ({
                    ...item,
                    availableNowFavorites: "calculating..."
                }));

                setSearchResults(updatedResults);

                for (const favorite of favorites) {
                    await new Promise(resolve => setTimeout(resolve, 1));
                    let url = new URL(`https://thunder.api.overdrive.com/v2/libraries/${favorite}/media/availability`);
                    await fetch(url, {
                        method: 'POST',
                        body: JSON.stringify({ids: mediaIds}),
                        headers: {'Content-Type': 'application/json'},
                    })
                        .then(response => response.json())
                        .then(availability => {
                            const newResults = updatedResults.map((result: any) => {
                                if (availability.items) {
                                    const item = availability.items.find((item2: any) => {
                                        if (item2 && item2.id) {
                                            return item2.id === "" + result.id;
                                        } else {
                                            return false;
                                        }
                                    });
                                    if (item && item.availableCopies > 0) {
                                        if (typeof result.availableNowFavorites === 'string') {
                                            result.availableNowFavorites = {
                                                libraryId: favorite,
                                                libraryName: favorite,
                                                libbyLink: `https://libbyapp.com/library/${favorite}/generated-36532/page-1/${item.id}`,
                                                totalLibraries: 0,
                                            };
                                        }
                                        if (item.availableCopies > (result.availableNowFavorites.mostAvailableCopies || 0)) {
                                            result.availableNowFavorites.libraryId = favorite;
                                            result.availableNowFavorites.libraryName = favorite;
                                            result.availableNowFavorites.libbyLink = `https://libbyapp.com/library/${favorite}/generated-36532/page-1/${item.id}`;
                                            result.availableNowFavorites.mostAvailableCopies = item.availableCopies;
                                            result.availableNowFavorites.totalLibraries += 1;
                                        }
                                        console.log('result', result);
                                    }
                                } else {
                                    console.error('Availability items are null or undefined');
                                }
                                return result;
                            });
                            setSearchResults([...newResults]);
                        })
                        .catch(error => {
                            console.error('Error:', error);
                        });
                }

                // Set "calculating..." results to "not found at favorites"
                const finalResults = updatedResults.map((result: any) => {
                    if (result.availableNowFavorites === "calculating...") {
                        result.availableNowFavorites = "not found at favorites";
                    }
                    return result;
                });
                setSearchResults([...finalResults]);

                setLoading(false);
            })
            .catch(error => {
                console.error('Error:', error);
                setLoading(false);
            });
    };

    useEffect(() => {
        const storedFavorites = JSON.parse(localStorage.getItem('favoriteIds') || '[]');
        setFavorites(storedFavorites);
    }, []);

    const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
        if (event.key === 'Enter') {
            handleSearch();
        }
    };

    return (
        <div>
            <SearchMedia></SearchMedia>
            <h2>Hardcover Search (want to read)</h2>
            <p>Searches your favorite libraries (see link above) for your want to read books from your <a
                href={"https://hardcover.app"}>hardcover</a> profile.
                It may take a minute if you have many favorite libraries.</p>
            <div style={{marginBottom: 20}}>
                <span style={{marginBottom: 20}}>
                    <label htmlFor="username">Hardcover user name:</label>
                    <input
                        id="hardcoveruser"
                        type="text"
                        placeholder="ex. freeeve"
                        value={username}
                        onChange={(e) => setUsername(e.target.value)}
                        onKeyDown={handleKeyDown}
                        style={{marginLeft: 10}}
                    />
                </span>
                <span style={{marginBottom: 20, marginLeft: 20}}>
                    <label htmlFor="additionalFilters">Additional filters:</label>
                    <input
                        id="additionalFilters"
                        type="text"
                        placeholder="ex. kindle english"
                        value={additionalFilters}
                        onChange={(e) => setAdditionalFilters(e.target.value)}
                        onKeyDown={handleKeyDown}
                        style={{marginLeft: 10}}
                    />
                </span>
                <button onClick={handleSearch} style={{marginLeft: 20}}>Search</button>
                {loading && <div className="spinner" style={{marginLeft: 10}}></div>}
            </div>
            <div className="ag-theme-alpine-dark" style={{height: 600, width: '100%'}}>
                <AgGridReact
                    columnDefs={columnDefs}
                    rowData={searchResults}
                    defaultColDef={{
                        sortable: true,
                        filter: true,
                        resizable: true
                    }}
                />
            </div>
        </div>
    );
}