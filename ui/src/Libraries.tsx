import {useState} from "react";
import {AgGridReact} from "ag-grid-react";

import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-alpine.css';
import {ColDef, SizeColumnsToFitGridStrategy} from "ag-grid-community";

interface Library {
    id: string;
    websiteId: number;
    name: string;
    isConsortium: boolean;
}

interface GridOptions {
    api: any;
}

export default function Libraries() {
    const baseUrl = 'http://localhost:8080/';
    //const baseUrl = window.location.origin;
    const [gridOptions, setGridOptions] = useState<GridOptions>({api: null});
    const [filteredRowCount, setFilteredRowCount] = useState(0);
    const [libraries, setLibraries] = useState<Library[]>([]);
    const columnDefs: ColDef[] = [
        {
            headerName: 'Name', field: 'name',
        },
        {
            headerName: 'Is Consortium?', field: 'isConsortium'
        },
        {
            headerName: 'Favorites',
            field: 'favorite',
            cellRenderer: (params: any) => {
                if (libraries.length > 0) {
                    if (getFavorites().includes(params.data.websiteId)) {
                        return (
                            <a onClick={() => removeFromFavorites(params.data.websiteId)}
                               style={{cursor: 'pointer'}}>
                                remove from favorites
                            </a>
                        );
                    } else {
                        return (
                            <a onClick={() => addToFavorites(params.data.websiteId)}
                               style={{cursor: 'pointer'}}>
                                add to favorites
                            </a>
                        );
                    }
                } else {
                    return null; // or some default JSX
                }
            }
        },
        {
            headerName: 'Open In Libby',
            field: 'libraryId',
            cellRenderer: (params: any) => {
                if (libraries.length > 0) {
                    return (
                        <a href={`https://libbyapp.com/library/${params.data.id}/`}
                           style={{cursor: 'pointer'}}>
                            open library in libby
                        </a>
                    );
                } else {
                    return null; // or some default JSX
                }
            }
        },
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

    const getFavorites = () => {
        return JSON.parse(localStorage.getItem('favorites') || '[]');
    }

    const addToFavorites = (websiteId: number) => {
        let favorites = getFavorites();
        if (favorites.includes(websiteId)) {
            return;
        }
        favorites.push(websiteId);
        localStorage.setItem('favorites', JSON.stringify(favorites));
        if (gridOptions && gridOptions.api) {
            gridOptions.api.redrawRows(); // Redraw the rows
        }
    }

    const removeFromFavorites = (websiteId: number) => {
        let favorites = getFavorites();
        favorites = favorites.filter((f: number) => f !== websiteId);
        localStorage.setItem('favorites', JSON.stringify(favorites));
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