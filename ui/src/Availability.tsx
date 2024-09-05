import {useParams} from "react-router-dom";
import {useEffect, useState} from "react";
import {AgGridReact} from "ag-grid-react";

import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-alpine.css';
import {ColDef, SizeColumnsToFitGridStrategy} from "ag-grid-community";
import SearchMedia from "./SearchMedia.tsx";

interface SelectedMedia {
    id: string;
    title: string;
    subtitle: string;
    creators: { name: string, role: string }[];
    languages: string[];
    formats: string[];
    description: string;
    coverUrl: string;
    seriesName: string;
    seriesReadOrder: number;
    libraryCount: number;
    availability: {
        library: { id: string, name: string, websiteId: number },
        ownedCount: number,
        availableCount: number,
        holdsCount: number,
        estimatedWaitDays: number
    }[];
}

interface Library {
    id: string;
    websiteId: number;
    name: string;
    isConsortium: boolean;
}

export default function Availability() {
    let baseUrl = window.location.origin;
    if (baseUrl === 'http://localhost:5173') {
        baseUrl = 'http://localhost:8080';
    }
    const {mediaId} = useParams();
    console.log(mediaId);
    const [selectedMedia, setSelectedMedia] = useState<SelectedMedia | null>(null);
    console.log("mediaId", mediaId);
    const [favorites, setFavorites] = useState<string[]>([]);

    useEffect(() => {
    }, [favorites, selectedMedia]);

    const columnDefs: ColDef[] = [
        {
            headerName: 'Library Name (click opens libby)', field: 'library.name', minWidth: 400,
            cellRenderer: (params: any) => {
                if (selectedMedia !== null) {
                    return (
                        <a href={`https://libbyapp.com/library/${params.data.library.id}/generated-36532/page-1/${selectedMedia.id}`}
                           style={{cursor: 'pointer'}}>
                            {params.value}
                        </a>
                    );
                } else {
                    return null; // or some default JSX
                }
            },
        },
        {
            headerName: 'Fav.', field: 'library.favorite', sort: 'desc', width: 130,
        },
        {headerName: 'Owned', field: 'ownedCount', width: 110},
        {headerName: 'Available', field: 'availableCount', sort: 'desc', width: 140},
        {headerName: 'Holds', field: 'holdsCount', width: 110},
        {headerName: 'Estimated Wait Days', field: 'estimatedWaitDays', sort: 'asc', width: 190},
        {headerName: 'Formats', field: 'formats', width: 190},
    ];

    const clickMedia = (selectedOption: any, favorites: string[]) => {
        let url = new URL('/api/availability', baseUrl);
        let params: any = {id: selectedOption.id};
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));
        // Fetch the availability data
        fetch(url, {
            method: 'GET',
        })
            .then((response) => response.json())
            .then((data) => {
                // Update the state with the selected book's details and availability data
                data.availability.forEach((item: any) => {
                    item.library.favorite = favorites.includes(item.library.id);
                });
                setSelectedMedia(data);
            })
            .catch((error) => {
                console.error('Error:', error);
            });
    };

    useEffect(() => {
        let startTime = new Date().getTime();
        let favorites = JSON.parse(localStorage.getItem('favoriteIds') || '[]');
        let oldFavorites = JSON.parse(localStorage.getItem('favorites') || '[]');
        if (favorites.length == 0 && oldFavorites.length > 0) {
            console.log('oldFavorites', oldFavorites, 'favorites', favorites);
            let url = new URL('/api/libraries', baseUrl);
            fetch(url, {
                method: 'GET',
            })
                .then((response) => response.json())
                .then((data) => {
                    let libraries = data.libraries;
                    oldFavorites.forEach((favWebsiteId: number) => {
                        libraries
                            .filter((l: Library) => l.websiteId === favWebsiteId)
                            .forEach((library: Library) => {
                                console.log('adding favorite', library.id, 'for websiteId', library.websiteId);
                                favorites.push(library.id);
                            });
                        localStorage.setItem('favoriteIds', JSON.stringify(favorites));
                    });
                    console.log('getFavorites took', new Date().getTime() - startTime, 'ms');
                    setFavorites(favorites);
                    if (mediaId) {
                        clickMedia({id: mediaId}, favorites);
                    }
                })
                .catch((error) => {
                    console.error('Error:', error);
                });
        } else {
            setFavorites(favorites);
            if (mediaId) {
                clickMedia({id: mediaId}, favorites);
            }
        }
    }, []);

    const autoSizeStrategy: SizeColumnsToFitGridStrategy = {
        type: 'fitGridWidth',
    };

    return (
        selectedMedia && (
            <div>
                <SearchMedia
                    clickMedia={clickMedia}
                ></SearchMedia>
                <h2>Media Availability</h2>
                <div style={{display: 'flex', justifyContent: 'space-between'}}>
                    <div>
                        <div style={{textAlign: 'left'}}>
                            <div><strong>Title:</strong> <strong>{selectedMedia.title}</strong></div>
                            {selectedMedia.seriesName !== "" && <div><strong>Series: </strong>
                                <strong>#{selectedMedia.seriesReadOrder} in {selectedMedia.seriesName}</strong></div>}
                            <div>
                                <strong>Creators:</strong> {selectedMedia.creators.map((author) => author.name + ' (' + author.role + ')').join(', ')}
                            </div>
                            <div><strong>Languages:</strong> {selectedMedia.languages.join(', ')}</div>
                            <div><strong>Formats (note that not all libraries have all formats--see format column
                                below):</strong>
                                <div>{selectedMedia.formats.join(', ')}</div>
                            </div>
                            {selectedMedia.subtitle != "" && (
                                <div><strong>Subtitle:</strong> {selectedMedia.subtitle}</div>)}
                            <div className={'dangerousHTML'}>
                                <div><strong>Description:</strong></div>
                                <div dangerouslySetInnerHTML={{__html: selectedMedia.description}}></div>
                            </div>
                            <div><a href={'https://www.overdrive.com/media/' + selectedMedia.id}>open in overdrive</a>
                            </div>
                        </div>
                    </div>
                    <div style={{textAlign: 'left'}}>
                        <span style={{
                            verticalAlign: 'top',
                            marginRight: 5
                        }}>owned by {selectedMedia.libraryCount} libraries</span>
                        <img src={selectedMedia.coverUrl}
                             alt={selectedMedia.title}
                             width={0} height={0}
                             sizes="100vw"
                             style={{width: 'auto', height: '100px'}} // optional
                        />
                    </div>
                </div>
                <div className="ag-theme-alpine-auto-dark" style={{height: 600, marginTop: 25}}>
                    <AgGridReact
                        columnDefs={columnDefs}
                        rowData={selectedMedia.availability}
                        defaultColDef={{
                            sortable: true,
                            filter: true,
                            resizable: true
                        }}
                        autoSizeStrategy={autoSizeStrategy}
                    />
                </div>
            </div>
        ));
}