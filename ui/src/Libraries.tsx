import {useEffect, useState} from "react";
import {AgGridReact} from "ag-grid-react";

import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-alpine.css';
import {ColDef, SizeColumnsToFitGridStrategy} from "ag-grid-community";
import SearchMedia from "./SearchMedia.tsx";

interface Library {
    id: string;
    websiteId: number;
    name: string;
    isConsortium: boolean;
    isAdvantageAccount: boolean;
}

interface GridOptions {
    api: any;
}

export default function Libraries() {
    //const baseUrl = 'http://localhost:8080/';
    let baseUrl = window.location.origin;
    if (baseUrl === 'http://localhost:5173') {
        baseUrl = 'http://localhost:8080';
    }
    const [gridOptions, setGridOptions] = useState<GridOptions>({api: null});
    const [filteredRowCount, setFilteredRowCount] = useState(0);
    const [libraries, setLibraries] = useState<Library[]>([]);
    const columnDefs: ColDef[] = [
        {
            headerName: 'Name (click opens libby)', field: 'name',
            cellRenderer: (params: any) => {
                if (libraries.length > 0) {
                    return (
                        <a href={`https://libbyapp.com/library/${params.data.id}/`}
                           style={{cursor: 'pointer'}}>
                            {params.value}
                        </a>
                    );
                } else {
                    return null; // or some default JSX
                }
            }
        },
        {
            headerName: 'Is Consortium?', field: 'isConsortium'
        },
        {
            headerName: 'Favorites',
            field: 'favdummy',
            valueGetter: (params: any) => {
                const isFavorite = getFavorites().includes(params.data.id) ? 'Favorite' : 'Not Favorite';
                console.log('isFavorite:', isFavorite);
                return isFavorite;
            },
            filter: false,
            cellRenderer: (params: any) => {
                if (libraries.length > 0) {
                    if (getFavorites().includes(params.data.id)) {
                        return (
                            <a onClick={() => removeFromFavorites(params.data.id)}
                               style={{cursor: 'pointer'}}>
                                remove from favorites
                            </a>
                        );
                    } else {
                        return (
                            <a onClick={() => addToFavorites(params.data.id)}
                               style={{cursor: 'pointer'}}>
                                add to favorites
                            </a>
                        );
                    }
                } else {
                    return null; // or some default JSX
                }
            }
        }
    ];

    const libraryOptions = () => {
        let url = new URL('/api/libraries', baseUrl);
        return fetch(url, {
            method: 'GET',
        })
            .then((response) => response.json())
            .then((data) => {
                data.libraries.sort((a: Library, b: Library) => a.name.localeCompare(b.name));
                setLibraries(data.libraries);
            })
            .catch((error) => {
                console.error('Error:', error);
            });
    };

    useEffect(() => {
        getFavorites();
    }, [libraries]);

    const getFavorites = () => {
        // let startTime = new Date().getTime();
        let favorites = JSON.parse(localStorage.getItem('favoriteIds') || '[]');
        let oldFavorites = JSON.parse(localStorage.getItem('favorites') || '[]');
        if (favorites.length == 0 && oldFavorites.length > 0 && libraries.length > 0) {
            // console.log('oldFavorites', oldFavorites, 'favorites', favorites);
            oldFavorites.forEach((favWebsiteId: number) => {
                libraries
                    .filter((l: Library) => l.websiteId === favWebsiteId)
                    .forEach((library: Library) => {
                        console.log('adding favorite', library.id, 'for websiteId', library.websiteId);
                        favorites.push(library.id);
                    });
            });
        }
        localStorage.setItem('favoriteIds', JSON.stringify(favorites));
        // console.log('getFavorites took', new Date().getTime() - startTime, 'ms');
        return favorites;
    }

    const addToFavorites = (libraryId: string) => {
        let favorites = getFavorites();
        if (favorites.includes(libraryId)) {
            return;
        }
        favorites.push(libraryId);
        localStorage.setItem('favoriteIds', JSON.stringify(favorites));
        if (gridOptions && gridOptions.api) {
            gridOptions.api.redrawRows(); // Redraw the rows
        }
    }

    const removeFromFavorites = (libraryId: string) => {
        let favorites = getFavorites();
        favorites = favorites.filter((f: string) => f !== libraryId);
        localStorage.setItem('favoriteIds', JSON.stringify(favorites));
        if (gridOptions && gridOptions.api) {
            gridOptions.api.redrawRows(); // Redraw the rows
        }
    }

    if (libraries.length === 0) {
        libraryOptions();
    }

    const autoSizeStrategy: SizeColumnsToFitGridStrategy = {
        type: 'fitGridWidth',
    };

    return (
        <div>
            <SearchMedia></SearchMedia>
            <h2>Favorite Libraries</h2>
            <div>
                {libraries.length && (
                    <div style={{marginTop: 25}}>
                        <div>{filteredRowCount} shown out of total {libraries.length}</div>
                        <div className="ag-theme-alpine-auto-dark" style={{height: 600, marginTop: 10}}>
                            <AgGridReact
                                columnDefs={columnDefs}
                                rowData={libraries}
                                defaultColDef={{
                                    sortable: true,
                                    filter: true,
                                    resizable: true
                                }}
                                autoSizeStrategy={autoSizeStrategy}
                                onFilterChanged={(params) => {
                                    setFilteredRowCount(params.api.getDisplayedRowCount());
                                }}
                                onGridReady={(params) => {
                                    setFilteredRowCount(params.api.getDisplayedRowCount());
                                    setGridOptions({api: params.api}); // Set the gridOptions state
                                }}
                                onRowDataUpdated={(params) => {
                                    setFilteredRowCount(params.api.getDisplayedRowCount());
                                }}
                            />
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}