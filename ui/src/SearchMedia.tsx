import {Virtuoso} from "react-virtuoso";
import {useState} from "react";
import {useNavigate} from "react-router-dom";

interface SearchMedia {
    id: string;
    title: string;
    creators: { name: string, role: string }[];
    languages: string[];
    formats: string[];
    coverUrl: string;
}

export default function SearchMedia() {
    // const baseUrl = 'http://localhost:8080/';
    const baseUrl = window.location.origin;
    const [data, setData] = useState({results: []});
    const navigate = useNavigate(); // Get the history object

    const loadSearchResults = (inputValue: string) => {
        let url = new URL('/api/search', baseUrl);
        let params: any = {q: inputValue};
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));

        fetch(url, {
            method: 'GET',
        })
            .then(response => response.json())
            .then(data => setData(data))
            .catch((error) => {
                console.error('Error:', error);
            });
    }

    const handleInputChange = (event: any) => {
        const value = event.target.value;
        if (value.length >= 3) {
            loadSearchResults(value);
        } else {
            setData({results: []});
        }
    };

    const Row = (index: number) => {
        const result: SearchMedia = data.results[index];
        console.log(index);
        return (
            <div onClick={() => navigate('/availability/' + result.id)}
                 style={{
                     backgroundColor: index % 2 === 0 ? '#333' : '#444',
                     display: 'flex',
                     justifyContent: 'space-between',
                     padding: 4
                 }}>
                <div style={{textAlign: 'left'}}>
                    <div><strong>{result.title}</strong></div>
                    <span>Creators: {result.creators.map((author) => author.name + ' (' + author.role + ')').join(', ')}</span>
                    <div>Languages: {result.languages.join(', ')}</div>
                    <div>Formats: {result.formats.join(', ')}</div>
                </div>
                <img src={result.coverUrl}
                     alt={result.title}
                     width={0} height={0}
                     sizes="100vw"
                     style={{width: 'auto', height: '100px'}} // optional
                />
            </div>
        );
    };
    return (
        <main>
            <div style={{width: '100%'}}>
                <div style={{fontSize: 42}}>DeepLibby Search</div>
                <input type="text"
                       placeholder="Search for a book (min 3 characters)"
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
        </main>
    );
}